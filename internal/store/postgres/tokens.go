package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// Одноразовые токены и очередь исходящих сообщений (F-03, F-22; DV-013).

// issueToken сохраняет токен и, если задано, сообщение в outbox. Прежние
// неиспользованные токены того же назначения аннулируются: действует только
// последняя ссылка.
func issueToken(ctx context.Context, tx pgx.Tx, userID int64, tok *domain.TokenSpec, msg *domain.Message, now time.Time) error {
	if tok != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE auth_tokens SET used_at = $3
			WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`, userID, tok.Purpose, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth_tokens (user_id, purpose, token_hash, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5)`, userID, tok.Purpose, tok.Hash, now, tok.Expires); err != nil {
			return err
		}
	}
	if msg != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO email_outbox (user_id, channel, kind, recipient, subject, body, next_attempt_at, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
			userID, msg.Channel, msg.Kind, msg.Recipient, msg.Subject, msg.Body, now); err != nil {
			return err
		}
	}
	return nil
}

// IssueToken сохраняет токен и сообщение в одной транзакции.
func (s *Store) IssueToken(ctx context.Context, userID int64, tok domain.TokenSpec, msg *domain.Message, now time.Time) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return issueToken(ctx, tx, userID, &tok, msg, now)
	})
}

// TokenStats возвращает время последнего токена назначения и число токенов
// за период since…now — для лимитов повторной отправки (REQ-SEC-005).
func (s *Store) TokenStats(ctx context.Context, userID int64, purpose string, since time.Time) (*time.Time, int, error) {
	var last *time.Time
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT max(created_at), count(*) FILTER (WHERE created_at >= $3)
		FROM auth_tokens WHERE user_id = $1 AND purpose = $2`, userID, purpose, since).Scan(&last, &n)
	return last, n, err
}

// useToken отмечает действующий токен использованным и возвращает его
// пользователя. Истёкший, использованный или неизвестный токен даёт
// domain.ErrTokenInvalid (REQ-SEC-006).
func useToken(ctx context.Context, tx pgx.Tx, purpose string, hash []byte, now time.Time) (int64, error) {
	var uid int64
	err := tx.QueryRow(ctx, `
		UPDATE auth_tokens SET used_at = $3
		WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > $3
		RETURNING user_id`, hash, purpose, now).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrTokenInvalid
	}
	return uid, err
}

// VerifyEmail подтверждает email по токену (REQ-FUNC-002).
func (s *Store) VerifyEmail(ctx context.Context, hash []byte, now time.Time) (int64, error) {
	var uid int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if uid, err = useToken(ctx, tx, domain.PurposeVerify, hash, now); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE users SET email_verified_at = coalesce(email_verified_at, $2) WHERE id = $1`, uid, now)
		return err
	})
	return uid, err
}

// ResetPassword задаёт новый пароль по токену и завершает все сессии
// пользователя (REQ-FUNC-004, REQ-SEC-003). Счётчик неудачных входов
// очищается, чтобы пользователь мог сразу войти.
func (s *Store) ResetPassword(ctx context.Context, hash []byte, passwordHash string, now time.Time) (int64, error) {
	var uid int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if uid, err = useToken(ctx, tx, domain.PurposeReset, hash, now); err != nil {
			return err
		}
		var email string
		if err = tx.QueryRow(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1 RETURNING email`, uid, passwordHash).Scan(&email); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, uid); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM login_failures WHERE email = $1`, email)
		return err
	})
	return uid, err
}

// LinkTelegram привязывает чат к пользователю по коду (REQ-FUNC-031). Если
// чат был привязан к другой учётной записи, он переходит к новой.
func (s *Store) LinkTelegram(ctx context.Context, hash []byte, chatID int64, now time.Time) (int64, error) {
	var uid int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if uid, err = useToken(ctx, tx, domain.PurposeTelegramLink, hash, now); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE users SET telegram_chat_id = NULL WHERE telegram_chat_id = $1 AND id <> $2`, chatID, uid); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE users SET telegram_chat_id = $2, telegram_enabled = true WHERE id = $1`, uid, chatID)
		return err
	})
	return uid, err
}

// UnlinkTelegramChat отвязывает чат (команда /stop или ответ 403 от Telegram).
func (s *Store) UnlinkTelegramChat(ctx context.Context, chatID int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET telegram_chat_id = NULL WHERE telegram_chat_id = $1`, chatID)
	return tag.RowsAffected() > 0, err
}

// UnlinkTelegram отвязывает Telegram пользователя (кнопка в профиле).
func (s *Store) UnlinkTelegram(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET telegram_chat_id = NULL WHERE id = $1`, userID)
	return err
}

// ClaimOutbox выбирает до limit записей, готовых к отправке, и откладывает
// их на lease, чтобы параллельный обработчик не отправил их повторно
// (FOR UPDATE SKIP LOCKED, DV-013).
func (s *Store) ClaimOutbox(ctx context.Context, channel string, now time.Time, limit int, lease time.Duration) ([]domain.OutboxItem, error) {
	var out []domain.OutboxItem
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, user_id, channel, kind, recipient, subject, body, attempts
			FROM email_outbox
			WHERE status = 'pending' AND channel = $1 AND next_attempt_at <= $2
			ORDER BY next_attempt_at, id
			LIMIT $3
			FOR UPDATE SKIP LOCKED`, channel, now, limit)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.OutboxItem, error) {
			var it domain.OutboxItem
			err := r.Scan(&it.ID, &it.UserID, &it.Channel, &it.Kind, &it.Recipient, &it.Subject, &it.Body, &it.Attempts)
			return it, err
		})
		if err != nil || len(out) == 0 {
			return err
		}
		ids := make([]int64, len(out))
		for i, it := range out {
			ids[i] = it.ID
		}
		_, err = tx.Exec(ctx, `UPDATE email_outbox SET next_attempt_at = $2 WHERE id = ANY($1)`, ids, now.Add(lease))
		return err
	})
	return out, err
}

// MarkSent отмечает запись отправленной.
func (s *Store) MarkSent(ctx context.Context, id int64, now time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE email_outbox SET status = 'sent', sent_at = $2, last_error = NULL WHERE id = $1`, id, now)
	return err
}

// MarkAttempt записывает неудачную попытку: следующая попытка в next или
// окончательная неудача (REQ-REL-005).
func (s *Store) MarkAttempt(ctx context.Context, id int64, attempts int, next time.Time, failed bool, errText string) error {
	status := "pending"
	if failed {
		status = "failed"
	}
	if len(errText) > 500 {
		errText = errText[:500]
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE email_outbox SET status = $2, attempts = $3, next_attempt_at = $4, last_error = $5 WHERE id = $1`,
		id, status, attempts, next, errText)
	return err
}

// RemoveFromWishlist удаляет игру из вишлиста (REQ-FUNC-014). После этого
// по игре не создаются уведомления пользователю: опрос и проверка условий
// идут по строкам wishlist_items.
func (s *Store) RemoveFromWishlist(ctx context.Context, userID int64, appID domain.AppID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM wishlist_items WHERE user_id = $1 AND appid = $2`, userID, int(appID))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotInWishlist
	}
	return err
}

// SetCondition меняет условие уведомления (REQ-FUNC-015). Если условие
// изменилось, last_notified_price сбрасывается (UC-07): новое условие
// применяется со следующей полученной цены.
func (s *Store) SetCondition(ctx context.Context, userID int64, appID domain.AppID, c domain.ItemCondition) error {
	var value any
	if c.Type == domain.ConditionMinPct || c.Type == domain.ConditionBelow {
		value = c.Value
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE wishlist_items SET cond_type = $3, cond_value = $4,
			last_notified_price = CASE WHEN cond_type = $3 AND cond_value IS NOT DISTINCT FROM $4::numeric
			                           THEN last_notified_price END
		WHERE user_id = $1 AND appid = $2`, userID, int(appID), string(c.Type), value)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		return domain.ErrInvalidCondition
	}
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotInWishlist
	}
	return err
}
