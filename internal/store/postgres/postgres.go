// Package postgres — репозитории на PostgreSQL (DV-004).
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/migrations"
)

// Store — доступ к базе данных.
type Store struct {
	pool *pgxpool.Pool
}

// Open подключается к БД с пулом maxConns соединений (DV-015).
func Open(ctx context.Context, url string, maxConns int32) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close закрывает пул.
func (s *Store) Close() { s.pool.Close() }

// Ping проверяет доступность БД для /healthz.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Migrate применяет миграции goose (REQ-INST-003).
func Migrate(ctx context.Context, url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, ".")
}

var _ = stdlib.GetDefaultDriver // регистрирует драйвер "pgx" для database/sql

// EnsureUser возвращает id пользователя с email, создавая его при отсутствии.
// В итерации 1 используется для временного демонстрационного пользователя,
// пока регистрация и сессии (F-02) не реализованы.
func (s *Store) EnsureUser(ctx context.Context, email string, region domain.Region) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, region_code) VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`, email, string(region)).Scan(&id)
	return id, err
}

// UserRegion возвращает регион пользователя.
func (s *Store) UserRegion(ctx context.Context, userID int64) (domain.Region, error) {
	var r string
	err := s.pool.QueryRow(ctx, `SELECT region_code FROM users WHERE id = $1`, userID).Scan(&r)
	return domain.Region(r), err
}

// WishlistStatus сообщает, есть ли игра в вишлисте, и сколько в нём игр.
func (s *Store) WishlistStatus(ctx context.Context, userID int64, appID domain.AppID) (exists bool, count int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT coalesce(bool_or(appid = $2), false), count(*)
		FROM wishlist_items WHERE user_id = $1`, userID, int(appID)).Scan(&exists, &count)
	return exists, count, err
}

// InWishlist возвращает подмножество appIDs, уже находящихся в вишлисте.
func (s *Store) InWishlist(ctx context.Context, userID int64, appIDs []domain.AppID) (map[domain.AppID]bool, error) {
	ids := make([]int64, len(appIDs))
	for i, a := range appIDs {
		ids[i] = int64(a)
	}
	rows, err := s.pool.Query(ctx, `SELECT appid FROM wishlist_items WHERE user_id = $1 AND appid = ANY($2)`, userID, ids)
	if err != nil {
		return nil, err
	}
	out := map[domain.AppID]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[domain.AppID(id)] = true
	}
	return out, rows.Err()
}

// AddToWishlist в одной транзакции сохраняет игру, текущую цену пары,
// точку истории (если цена новая или изменилась, ADR-0004) и элемент вишлиста.
func (s *Store) AddToWishlist(ctx context.Context, userID int64, region domain.Region, g domain.Game, p domain.Price, now time.Time) (domain.WishlistItem, error) {
	item := domain.WishlistItem{Game: g, Region: region, Condition: domain.ConditionAny, Price: p, CheckedAt: now, AddedAt: now}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO games (appid, name, cover_url) VALUES ($1, $2, $3)
			ON CONFLICT (appid) DO UPDATE SET name = EXCLUDED.name, cover_url = EXCLUDED.cover_url`,
			int(g.AppID), g.Name, g.CoverURL); err != nil {
			return err
		}
		if err := upsertPrice(ctx, tx, g.AppID, region, p, now, "steam"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO wishlist_items (user_id, appid, added_at) VALUES ($1, $2, $3)`,
			userID, int(g.AppID), now)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAlreadyInWishlist
		}
		return err
	})
	return item, err
}

func nullable(v int, ok bool) any {
	if !ok {
		return nil
	}
	return v
}

func upsertPrice(ctx context.Context, tx pgx.Tx, app domain.AppID, region domain.Region, p domain.Price, now time.Time, source string) error {
	var prev domain.Price
	var final, initial sql.NullInt32
	var currency sql.NullString
	err := tx.QueryRow(ctx, `
		SELECT availability, final_price, initial_price, discount_pct, currency
		FROM price_state WHERE appid = $1 AND region_code = $2 FOR UPDATE`,
		int(app), string(region)).Scan(&prev.Availability, &final, &initial, &prev.DiscountPct, &currency)
	first := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !first {
		return err
	}
	prev.Final, prev.Initial, prev.Currency = int(final.Int32), int(initial.Int32), currency.String

	priced := p.Availability == domain.Available
	if _, err := tx.Exec(ctx, `
		INSERT INTO price_state (appid, region_code, availability, final_price, initial_price, discount_pct, currency,
		                         hist_low_price, hist_low_at, history_since, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $4, CASE WHEN $4::int IS NULL THEN NULL ELSE $8::timestamptz END, $8, $8)
		ON CONFLICT (appid, region_code) DO UPDATE SET
			availability = EXCLUDED.availability, final_price = EXCLUDED.final_price,
			initial_price = EXCLUDED.initial_price, discount_pct = EXCLUDED.discount_pct,
			currency = EXCLUDED.currency, checked_at = EXCLUDED.checked_at,
			hist_low_at = CASE WHEN EXCLUDED.final_price < coalesce(price_state.hist_low_price, 2147483647)
			                   THEN EXCLUDED.checked_at ELSE price_state.hist_low_at END,
			hist_low_price = CASE WHEN EXCLUDED.final_price < coalesce(price_state.hist_low_price, 2147483647)
			                      THEN EXCLUDED.final_price ELSE price_state.hist_low_price END`,
		int(app), string(region), string(p.Availability), nullable(p.Final, priced), nullable(p.Initial, priced),
		p.DiscountPct, nullStr(p.Currency), now); err != nil {
		return err
	}
	if !first && !p.Changed(prev) {
		return nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO price_history (appid, region_code, availability, final_price, initial_price, discount_pct, currency, recorded_at, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (appid, region_code, recorded_at, source) DO NOTHING`,
		int(app), string(region), string(p.Availability), nullable(p.Final, priced), nullable(p.Initial, priced),
		p.DiscountPct, nullStr(p.Currency), now, source)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Wishlist возвращает вишлист пользователя с ценами в его регионе, новые сверху.
func (s *Store) Wishlist(ctx context.Context, userID int64) ([]domain.WishlistItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.appid, g.name, g.cover_url, u.region_code, w.cond_type, w.added_at,
		       coalesce(ps.availability, 'unavailable'), coalesce(ps.final_price, 0), coalesce(ps.initial_price, 0),
		       coalesce(ps.discount_pct, 0), coalesce(ps.currency, ''), coalesce(ps.checked_at, w.added_at)
		FROM wishlist_items w
		JOIN users u ON u.id = w.user_id
		JOIN games g ON g.appid = w.appid
		LEFT JOIN price_state ps ON ps.appid = w.appid AND ps.region_code = u.region_code
		WHERE w.user_id = $1
		ORDER BY w.added_at DESC, g.appid`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.WishlistItem
	for rows.Next() {
		var it domain.WishlistItem
		var app int
		var region, cond, avail string
		if err := rows.Scan(&app, &it.Name, &it.CoverURL, &region, &cond, &it.AddedAt, &avail,
			&it.Price.Final, &it.Price.Initial, &it.Price.DiscountPct, &it.Price.Currency, &it.CheckedAt); err != nil {
			return nil, err
		}
		it.AppID, it.Region, it.Condition, it.Price.Availability = domain.AppID(app), domain.Region(region), domain.Condition(cond), domain.Availability(avail)
		out = append(out, it)
	}
	return out, rows.Err()
}

// HistoryCount возвращает число точек истории пары (для тестов и диагностики).
func (s *Store) HistoryCount(ctx context.Context, app domain.AppID, region domain.Region) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM price_history WHERE appid = $1 AND region_code = $2`,
		int(app), string(region)).Scan(&n)
	return n, err
}

// LastCycleFinishedAt — время окончания последнего успешного цикла для /healthz.
func (s *Store) LastCycleFinishedAt(ctx context.Context) (*time.Time, error) {
	var t *time.Time
	err := s.pool.QueryRow(ctx, `SELECT max(finished_at) FROM poll_cycles`).Scan(&t)
	if err != nil {
		return nil, fmt.Errorf("last cycle: %w", err)
	}
	return t, nil
}
