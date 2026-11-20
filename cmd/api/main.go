// Команда api — API-сервер (DV-002): REST /api/v1 и /healthz.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kiryshabutor/SDLC/internal/app"
	"github.com/kiryshabutor/SDLC/internal/httpapi"
	"github.com/kiryshabutor/SDLC/internal/i18n"
	"github.com/kiryshabutor/SDLC/internal/platform"
	"github.com/kiryshabutor/SDLC/internal/source"
	"github.com/kiryshabutor/SDLC/internal/source/mock"
	"github.com/kiryshabutor/SDLC/internal/source/steam"
	"github.com/kiryshabutor/SDLC/internal/store/postgres"
	"github.com/kiryshabutor/SDLC/pkg/steamstore"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}
	log := platform.NewLogger(os.Stdout, "api", cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := postgres.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	store, err := postgres.Open(ctx, cfg.DatabaseURL, 10)
	if err != nil {
		return err
	}
	defer store.Close()

	var games source.GameSearch
	if cfg.TestMode {
		games = mock.New()
	} else {
		client := steamstore.New()
		client.BaseURL = cfg.SteamBaseURL
		games = steam.New(client, platform.NewWindow(cfg.APISteamQuota, 5*time.Minute))
	}

	accounts := &app.Accounts{Store: store, Secrets: app.Secrets{
		HashPassword:  platform.HashPassword,
		CheckPassword: platform.CheckPassword,
		DummyHash:     platform.DummyPasswordHash,
		NewToken:      platform.NewToken,
		HashToken:     platform.HashToken,
	}}
	srv := &httpapi.Server{
		Accounts: accounts,
		Wishlist: &app.Wishlist{Store: store, Search: games},
		Health:   store,
		Log:      log,
		Msg:      i18n.MustLoad(i18n.DefaultLang),
	}
	hs := &http.Server{Addr: cfg.HTTPAddr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}

	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	log.Info("api started", "addr", cfg.HTTPAddr, "test_mode", cfg.TestMode)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hs.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("api stopped")
	return nil
}
