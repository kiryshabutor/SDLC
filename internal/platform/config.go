// Package platform — инфраструктурные части, общие для cmd/api и cmd/worker:
// конфигурация из окружения, журналы, ограничитель частоты, повторы, версия.
package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version подставляется при сборке: -ldflags "-X …/platform.Version=v0.1.0" (REQ-BUILD-003).
var Version = "dev"

// Config — параметры из переменных окружения (REQ-INST-002). Полный список
// с комментариями — в .env.example.
type Config struct {
	DatabaseURL   string
	HTTPAddr      string
	PollInterval  time.Duration
	SteamBaseURL  string
	TestMode      bool
	LogLevel      string
	APISteamQuota int

	// AppBaseURL — внешний адрес веб-клиента для ссылок в письмах.
	AppBaseURL string
	// SMTP (REQ-INT-010). Пустой SMTPHost — письма копятся в outbox.
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPTLS      string
	// Telegram (REQ-INT-012). Пустой токен — функции Telegram выключены.
	TelegramToken  string
	TelegramBot    string
	TelegramAPIURL string
}

// LoadConfig читает окружение и проверяет значения.
func LoadConfig() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		SteamBaseURL:  env("STEAM_STORE_URL", "https://store.steampowered.com"),
		LogLevel:      env("LOG_LEVEL", "info"),
		APISteamQuota: 30,

		AppBaseURL:     env("APP_BASE_URL", "http://localhost:8080"),
		SMTPHost:       os.Getenv("SMTP_HOST"),
		SMTPUser:       os.Getenv("SMTP_USER"),
		SMTPPassword:   os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:       env("SMTP_FROM", "Охотник за скидками <noreply@localhost>"),
		SMTPTLS:        env("SMTP_TLS", "starttls"),
		TelegramToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramBot:    strings.TrimPrefix(os.Getenv("TELEGRAM_BOT_NAME"), "@"),
		TelegramAPIURL: env("TELEGRAM_API_URL", "https://api.telegram.org"),
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	var err error
	if c.PollInterval, err = time.ParseDuration(env("POLL_INTERVAL", "3h")); err != nil {
		return c, fmt.Errorf("POLL_INTERVAL: %w", err)
	}
	if c.PollInterval < time.Hour || c.PollInterval > 24*time.Hour {
		return c, fmt.Errorf("POLL_INTERVAL must be between 1h and 24h, got %s", c.PollInterval)
	}
	if c.TestMode, err = strconv.ParseBool(env("TEST_MODE", "false")); err != nil {
		return c, fmt.Errorf("TEST_MODE: %w", err)
	}
	if v := os.Getenv("API_STEAM_QUOTA"); v != "" {
		if c.APISteamQuota, err = strconv.Atoi(v); err != nil || c.APISteamQuota <= 0 {
			return c, fmt.Errorf("API_STEAM_QUOTA must be a positive integer")
		}
	}
	if c.SMTPPort, err = strconv.Atoi(env("SMTP_PORT", "587")); err != nil || c.SMTPPort <= 0 {
		return c, fmt.Errorf("SMTP_PORT must be a positive integer")
	}
	if (c.TelegramToken == "") != (c.TelegramBot == "") {
		return c, fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_BOT_NAME must be set together")
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
