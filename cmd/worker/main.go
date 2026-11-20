// Команда worker — фоновый обработчик (DV-002): отправка писем из outbox
// (DV-013), бот Telegram (привязка по коду), ежесуточная очистка истёкших
// сессий (DV-004), таймер POLL_INTERVAL и корректное завершение по SIGTERM.
// Цикл опроса цен добавляется в итерации 4 (F-08).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kiryshabutor/SDLC/internal/app"
	"github.com/kiryshabutor/SDLC/internal/i18n"
	"github.com/kiryshabutor/SDLC/internal/mail"
	"github.com/kiryshabutor/SDLC/internal/platform"
	"github.com/kiryshabutor/SDLC/internal/store/postgres"
	"github.com/kiryshabutor/SDLC/internal/telegram"
	tgapi "github.com/kiryshabutor/SDLC/pkg/telegram"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}
	log := platform.NewLogger(os.Stdout, "worker", cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := waitForDB(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return err
	}
	defer store.Close()
	log.Info("worker started", "poll_interval", cfg.PollInterval.String(), "test_mode", cfg.TestMode)
	texts := i18n.MustLoad(i18n.DefaultLang)

	// Письма (F-03). Без SMTP письма остаются в очереди до настройки сервера.
	if cfg.SMTPHost == "" {
		log.Warn("SMTP_HOST is empty: emails stay in outbox until SMTP is configured")
	} else {
		mc := mail.Config{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword,
			From: cfg.SMTPFrom, TLS: cfg.SMTPTLS}
		if err := mc.Validate(); err != nil {
			return err
		}
		ob := &app.Outbox{Store: store, Notifier: &mail.SMTP{Cfg: mc}, Log: log.With("part", "outbox")}
		go ob.Run(ctx)
		log.Info("email outbox started", "smtp_host", cfg.SMTPHost, "smtp_tls", cfg.SMTPTLS)
	}

	// Бот Telegram (F-22): привязка по коду и /stop. Без токена выключен.
	if cfg.TelegramToken != "" {
		client := tgapi.New(cfg.TelegramToken)
		client.BaseURL = cfg.TelegramAPIURL
		bot := &telegram.Bot{API: client, Store: store, HashToken: platform.HashToken, Texts: texts, Log: log.With("part", "telegram")}
		go bot.Run(ctx)
		log.Info("telegram bot started", "bot", cfg.TelegramBot)
	}

	purge := func() {
		n, err := store.PurgeExpired(ctx, time.Now().UTC())
		if err != nil {
			log.Error("purge failed", "err", err)
			return
		}
		log.Info("expired sessions purged", "rows", n)
	}
	purge()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()
	daily := time.NewTicker(24 * time.Hour)
	defer daily.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped")
			return nil
		case <-daily.C:
			purge()
		case <-ticker.C:
			cycle := platform.NewRequestID()
			log.Info("poll cycle skipped: not implemented until F-08", "cycle_id", cycle)
		}
	}
}

// waitForDB ждёт готовности БД при старте контейнеров (REQ-AVAIL-002).
func waitForDB(ctx context.Context, url string, log *slog.Logger) (*postgres.Store, error) {
	for attempt := 1; ; attempt++ {
		store, err := postgres.Open(ctx, url, 4)
		if err == nil {
			if err = store.Ping(ctx); err == nil {
				return store, nil
			}
			store.Close()
		}
		log.Warn("database not ready", "attempt", attempt, "err", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
