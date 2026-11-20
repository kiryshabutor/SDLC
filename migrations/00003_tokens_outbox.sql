-- Итерация 3 (F-03, F-22): одноразовые токены и очередь исходящих сообщений.

-- +goose Up
-- Токены подтверждения email, сброса пароля, привязки Telegram и (с F-10)
-- отписки. В БД хранится только SHA-256 токена (REQ-SEC-006).
CREATE TABLE auth_tokens (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text NOT NULL CHECK (purpose IN ('verify', 'reset', 'unsubscribe', 'telegram_link')),
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);
CREATE INDEX auth_tokens_user_purpose ON auth_tokens (user_id, purpose, created_at DESC);

-- Очередь исходящих сообщений для обоих внешних каналов (DV-013, ADR-0007,
-- SM-02). Запись создаётся в одной транзакции с событием и отправляется
-- фоновым обработчиком; до 3 попыток с интервалом 5 минут (REQ-REL-005).
CREATE TABLE email_outbox (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    channel         text NOT NULL CHECK (channel IN ('email', 'telegram')),
    kind            text NOT NULL CHECK (kind IN ('verify', 'reset', 'digest')),
    recipient       text NOT NULL,
    subject         text NOT NULL DEFAULT '',
    body            text NOT NULL,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts        smallint NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL,
    last_error      text,
    created_at      timestamptz NOT NULL,
    sent_at         timestamptz
);
CREATE INDEX email_outbox_pending ON email_outbox (next_attempt_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE email_outbox;
DROP TABLE auth_tokens;
