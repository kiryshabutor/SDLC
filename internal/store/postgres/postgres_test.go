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
	if _, err := s.pool.Exec(ctx, `TRUNCATE sessions, login_failures, wishlist_items, price_history, price_state, games, users, poll_cycles RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	return s
}

// newUser создаёт пользователя с фиктивным хэшем пароля.
func newUser(t *testing.T, s *Store, email string, region domain.Region) int64 {
	t.Helper()
	id, err := s.CreateUser(context.Background(), email, "h", region, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAddToWishlistAndHistory(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u1 := newUser(t, s, "a@example.com", domain.RegionBY)
	u2 := newUser(t, s, "b@example.com", domain.RegionBY)

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
	if err != nil || len(items) != 1 || items[0].Price == nil || items[0].Price.Final != 5999 || items[0].Condition != domain.ConditionAny {
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
