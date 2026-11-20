package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// Учётные записи и сессии (F-02, F-04, F-17; DV-004, ADR-0005).

// CreateUser создаёт пользователя. Занятый email даёт domain.ErrEmailTaken.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string, region domain.Region, consentAt time.Time) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, region_code, pd_consent_at, created_at)
		VALUES ($1, $2, $3, $4, $4) RETURNING id`,
		email, passwordHash, string(region), consentAt).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, domain.ErrEmailTaken
	}
	return id, err
}

// Credentials возвращает id и хэш пароля по email; нет пользователя —
// domain.ErrUserNotFound.
func (s *Store) Credentials(ctx context.Context, email string) (int64, string, error) {
	var id int64
	var hash *string
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash FROM users WHERE email = $1`, email).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && hash == nil) {
		return 0, "", domain.ErrUserNotFound
	}
	if err != nil {
		return 0, "", err
	}
	return id, *hash, nil
}

// PasswordHash возвращает хэш пароля пользователя.
func (s *Store) PasswordHash(ctx context.Context, userID int64) (string, error) {
	var hash *string
	err := s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && hash == nil) {
		return "", domain.ErrUserNotFound
	}
	if err != nil {
		return "", err
	}
	return *hash, nil
}

// RecentLoginFailures возвращает время последних n неудачных попыток входа
// для email, от новых к старым.
func (s *Store) RecentLoginFailures(ctx context.Context, email string, n int) ([]time.Time, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT failed_at FROM login_failures WHERE email = $1
		ORDER BY failed_at DESC LIMIT $2`, email, n)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[time.Time])
}

// RecordLoginFailure записывает неудачную попытку входа.
func (s *Store) RecordLoginFailure(ctx context.Context, email string, at time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO login_failures (email, failed_at) VALUES ($1, $2)`, email, at)
	return err
}

// ClearLoginFailures удаляет историю неудачных попыток после успешного входа.
func (s *Store) ClearLoginFailures(ctx context.Context, email string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM login_failures WHERE email = $1`, email)
	return err
}

// CreateSession сохраняет сессию; в БД попадает только хэш идентификатора.
func (s *Store) CreateSession(ctx context.Context, idHash []byte, userID int64, now, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id_hash, user_id, created_at, last_seen_at, expires_at)
		VALUES ($1, $2, $3, $3, $4)`, idHash, userID, now, expires)
	return err
}

// TouchSession продлевает действующую сессию и возвращает её пользователя.
// Истёкшая или неизвестная сессия даёт domain.ErrUserNotFound.
func (s *Store) TouchSession(ctx context.Context, idHash []byte, now, expires time.Time) (int64, error) {
	var uid int64
	err := s.pool.QueryRow(ctx, `
		UPDATE sessions SET last_seen_at = $2, expires_at = $3
		WHERE id_hash = $1 AND expires_at > $2
		RETURNING user_id`, idHash, now, expires).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrUserNotFound
	}
	return uid, err
}

// DeleteSession завершает одну сессию (выход).
func (s *Store) DeleteSession(ctx context.Context, idHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id_hash = $1`, idHash)
	return err
}

// DeleteUserSessions завершает все сессии пользователя (сброс пароля, F-03).
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

// Profile возвращает профиль и настройки пользователя.
func (s *Store) Profile(ctx context.Context, userID int64) (domain.Profile, error) {
	var p domain.Profile
	var region string
	var verified *time.Time
	var chat *int64
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, region_code, email_verified_at, email_enabled, telegram_chat_id, telegram_enabled, created_at
		FROM users WHERE id = $1`, userID).
		Scan(&p.ID, &p.Email, &region, &verified, &p.EmailEnabled, &chat, &p.TelegramEnabled, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, domain.ErrUserNotFound
	}
	p.Region, p.EmailVerified, p.TelegramLinked = domain.Region(region), verified != nil, chat != nil
	return p, err
}

// CountConditions возвращает число условий данного типа в вишлисте.
func (s *Store) CountConditions(ctx context.Context, userID int64, cond domain.Condition) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM wishlist_items WHERE user_id = $1 AND cond_type = $2`,
		userID, string(cond)).Scan(&n)
	return n, err
}

// UpdateProfile применяет изменения в одной транзакции. При смене региона
// условия «цена не выше X» сбрасываются на «любая скидка», а последняя
// уведомлённая цена обнуляется, так как она в старой валюте (REQ-FUNC-005).
func (s *Store) UpdateProfile(ctx context.Context, userID int64, region *domain.Region, emailEnabled, telegramEnabled *bool) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var old string
		if err := tx.QueryRow(ctx, `SELECT region_code FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&old); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrUserNotFound
			}
			return err
		}
		if region != nil && string(*region) != old {
			if _, err := tx.Exec(ctx, `UPDATE users SET region_code = $2 WHERE id = $1`, userID, string(*region)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE wishlist_items SET cond_type = 'any', cond_value = NULL WHERE user_id = $1 AND cond_type = 'below'`,
				userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE wishlist_items SET last_notified_price = NULL WHERE user_id = $1`, userID); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			UPDATE users SET email_enabled = coalesce($2, email_enabled), telegram_enabled = coalesce($3, telegram_enabled)
			WHERE id = $1`, userID, emailEnabled, telegramEnabled)
		return err
	})
}

// DeleteUser безвозвратно удаляет пользователя. Сессии, вишлист и (с F-09)
// уведомления удаляются каскадно; журнал попыток входа хранится по email и
// удаляется отдельно. Общие таблицы цен не затрагиваются (REQ-FUNC-007).
func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var email string
		err := tx.QueryRow(ctx, `DELETE FROM users WHERE id = $1 RETURNING email`, userID).Scan(&email)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUserNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM login_failures WHERE email = $1`, email)
		return err
	})
}

// PurgeExpired удаляет истёкшие сессии и старые записи о попытках входа
// (ежесуточная очистка, DV-004). Возвращает число удалённых строк.
func (s *Store) PurgeExpired(ctx context.Context, now time.Time) (int64, error) {
	a, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	b, err := s.pool.Exec(ctx, `DELETE FROM login_failures WHERE failed_at < $1`, now.Add(-24*time.Hour))
	if err != nil {
		return 0, err
	}
	return a.RowsAffected() + b.RowsAffected(), nil
}
