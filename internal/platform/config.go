// Package platform — инфраструктурные части, общие для cmd/api и cmd/worker:
// конфигурация из окружения, журналы, ограничитель частоты, повторы, версия.
package platform

import (
	"fmt"
	"os"
	"strconv"
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
	DevUserEmail  string
	APISteamQuota int
}

// LoadConfig читает окружение и проверяет значения.
func LoadConfig() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		SteamBaseURL:  env("STEAM_STORE_URL", "https://store.steampowered.com"),
		LogLevel:      env("LOG_LEVEL", "info"),
		DevUserEmail:  env("DEV_USER_EMAIL", "demo@example.com"),
		APISteamQuota: 30,
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
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
