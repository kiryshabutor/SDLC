package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// Интеграционные тесты выполняются при заданной TEST_DATABASE_URL
// (в CI — сервис postgres:16-alpine). База очищается перед тестом.
func openTest(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задана")
	}
	ctx := context.Background()
	if err := Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, url, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if _, err := s.pool.Exec(ctx, `TRUNCATE wishlist_items, price_history, price_state, games, users, poll_cycles RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAddToWishlistAndHistory(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u1, err := s.EnsureUser(ctx, "a@example.com", domain.RegionBY)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := s.EnsureUser(ctx, "a@example.com", domain.RegionBY); again != u1 {
		t.Fatalf("EnsureUser не идемпотентен: %d != %d", again, u1)
	}
	u2, _ := s.EnsureUser(ctx, "b@example.com", domain.RegionBY)

	g := domain.Game{AppID: 1245620, Name: "ELDEN RING"}
	p := domain.Price{Availability: domain.Available, Final: 5999, Initial: 5999, Currency: "USD"}
	t0 := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

	if _, err := s.AddToWishlist(ctx, u1, domain.RegionBY, g, p, t0); err != nil {
		t.Fatal(err)
	}
	// Второй пользователь с той же ценой: новой точки истории нет (ADR-0004).
	if _, err := s.AddToWishlist(ctx, u2, domain.RegionBY, g, p, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.HistoryCount(ctx, g.AppID, domain.RegionBY); n != 1 {
		t.Fatalf("history = %d; want 1", n)
	}
	if _, err := s.AddToWishlist(ctx, u1, domain.RegionBY, g, p, t0); !errors.Is(err, domain.ErrAlreadyInWishlist) {
		t.Fatalf("дубликат: err = %v", err)
	}

	exists, count, err := s.WishlistStatus(ctx, u1, g.AppID)
	if err != nil || !exists || count != 1 {
		t.Fatalf("status: %v %d %v", exists, count, err)
	}
	items, err := s.Wishlist(ctx, u1)
	if err != nil || len(items) != 1 || items[0].Price.Final != 5999 || items[0].Condition != domain.ConditionAny {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	in, _ := s.InWishlist(ctx, u1, []domain.AppID{1245620, 570})
	if !in[1245620] || in[570] {
		t.Fatalf("InWishlist = %v", in)
	}
}

func TestHealth(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if err := s.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if last, err := s.LastCycleFinishedAt(ctx); err != nil || last != nil {
		t.Fatalf("last=%v err=%v", last, err)
	}
}
