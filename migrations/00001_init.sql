-- Итерация 1 (F-01, F-05): таблицы для учётных записей, игр, цен и вишлиста.
-- Схема соответствует DV-004; таблицы сессий, уведомлений и outbox
-- добавляются миграциями своих функций (F-02, F-09, F-03).

-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE regions (
    code           text PRIMARY KEY,
    name           text NOT NULL,
    currency       text NOT NULL,
    itad_supported boolean NOT NULL DEFAULT false
);

INSERT INTO regions (code, name, currency, itad_supported) VALUES
    ('BY', 'Беларусь', 'USD', false),
    ('RU', 'Россия', 'RUB', false),
    ('KZ', 'Казахстан', 'USD', false),
    ('US', 'США', 'USD', true),
    ('PL', 'Польша', 'PLN', true);

CREATE TABLE users (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email             citext NOT NULL UNIQUE,
    password_hash     text,              -- заполняется регистрацией (F-02)
    region_code       text NOT NULL REFERENCES regions (code),
    email_verified_at timestamptz,
    email_enabled     boolean NOT NULL DEFAULT true,
    telegram_chat_id  bigint UNIQUE,
    telegram_enabled  boolean NOT NULL DEFAULT true,
    steam_profile_url text,
    steam_id64        text,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE games (
    appid     integer PRIMARY KEY,
    itad_id   text,
    name      text NOT NULL,
    cover_url text NOT NULL DEFAULT ''
);

CREATE TABLE price_state (
    appid          integer NOT NULL REFERENCES games (appid),
    region_code    text NOT NULL REFERENCES regions (code),
    availability   text NOT NULL CHECK (availability IN ('available', 'unavailable', 'free', 'unreleased')),
    final_price    integer,
    initial_price  integer,
    discount_pct   smallint NOT NULL DEFAULT 0,
    currency       text,
    hist_low_price integer,
    hist_low_at    timestamptz,
    history_since  timestamptz,
    checked_at     timestamptz NOT NULL,
    PRIMARY KEY (appid, region_code)
);

CREATE TABLE price_history (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    appid         integer NOT NULL REFERENCES games (appid),
    region_code   text NOT NULL REFERENCES regions (code),
    availability  text NOT NULL,
    final_price   integer,
    initial_price integer,
    discount_pct  smallint NOT NULL DEFAULT 0,
    currency      text,
    recorded_at   timestamptz NOT NULL,
    source        text NOT NULL CHECK (source IN ('steam', 'itad', 'test'))
);
CREATE UNIQUE INDEX price_history_dedup ON price_history (appid, region_code, recorded_at, source);

CREATE TABLE wishlist_items (
    user_id             bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    appid               integer NOT NULL REFERENCES games (appid),
    cond_type           text NOT NULL DEFAULT 'any' CHECK (cond_type IN ('any', 'min_pct', 'below', 'hist_low')),
    cond_value          numeric,
    last_notified_price integer,
    added_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, appid)
);
CREATE INDEX wishlist_items_appid ON wishlist_items (appid);

CREATE TABLE poll_cycles (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind        text NOT NULL CHECK (kind IN ('scheduled', 'manual', 'test')),
    started_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    stats       jsonb NOT NULL DEFAULT '{}'
);

-- +goose Down
DROP TABLE poll_cycles;
DROP TABLE wishlist_items;
DROP TABLE price_history;
DROP TABLE price_state;
DROP TABLE games;
DROP TABLE users;
DROP TABLE regions;
