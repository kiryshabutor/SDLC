// Команда worker — фоновый обработчик (DV-002). В итерации 1 это каркас:
// конфигурация, журнал, подключение к БД, таймер POLL_INTERVAL и корректное
// завершение по SIGTERM. Цикл опроса цен добавляется в итерации 4 (F-08).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kiryshabutor/SDLC/internal/platform"
	"github.com/kiryshabutor/SDLC/internal/store/postgres"
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

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped")
			return nil
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
