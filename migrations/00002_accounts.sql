-- Итерация 2 (F-02): учётные записи, серверные сессии (ADR-0005) и журнал
-- неудачных попыток входа для блокировки перебора (REQ-SEC-005).
-- Таблица auth_tokens (подтверждение email, сброс пароля) добавляется в F-03.

-- +goose Up
-- Время согласия на обработку персональных данных (REQ-COMP-001).
-- У пользователей, созданных до регистрации (демонстрационный пользователь
-- итерации 1), значение пустое: войти они не могут, так как у них нет пароля.
ALTER TABLE users ADD COLUMN pd_consent_at timestamptz;

-- Временный демонстрационный пользователь итерации 1 больше не нужен.
DELETE FROM users WHERE password_hash IS NULL;

CREATE TABLE sessions (
    id_hash      bytea PRIMARY KEY,          -- SHA-256 от значения cookie sid
    user_id      bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL        -- last_seen_at + 30 дней
);
CREATE INDEX sessions_user_id ON sessions (user_id);
CREATE INDEX sessions_expires_at ON sessions (expires_at);

-- Записи хранятся по email, а не по user_id: блокировка действует и для
-- незарегистрированных адресов, иначе по ней можно было бы определить,
-- зарегистрирован ли email (REQ-SEC-007).
CREATE TABLE login_failures (
    email     citext NOT NULL,
    failed_at timestamptz NOT NULL
);
CREATE INDEX login_failures_email ON login_failures (email, failed_at DESC);

-- +goose Down
DROP TABLE login_failures;
DROP TABLE sessions;
ALTER TABLE users DROP COLUMN pd_consent_at;
