package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

func TestUsersAndCredentials(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	id, err := s.CreateUser(ctx, "kirill@example.com", "$argon2id$x", domain.RegionBY, t0)
	if err != nil {
		t.Fatal(err)
	}
	// citext: email уникален без учёта регистра.
	if _, err := s.CreateUser(ctx, "KIRILL@example.com", "h", domain.RegionBY, t0); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("дубликат: %v", err)
	}
	gotID, hash, err := s.Credentials(ctx, "kirill@example.com")
	if err != nil || gotID != id || hash != "$argon2id$x" {
		t.Fatalf("%d %q %v", gotID, hash, err)
	}
	if _, _, err := s.Credentials(ctx, "nobody@example.com"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("нет пользователя: %v", err)
	}
	p, err := s.Profile(ctx, id)
	if err != nil || p.Email != "kirill@example.com" || p.Region != domain.RegionBY || !p.EmailEnabled ||
		p.EmailVerified || p.TelegramLinked || !p.CreatedAt.Equal(t0) {
		t.Fatalf("profile=%+v err=%v", p, err)
	}
}

func TestSessions(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	uid := newUser(t, s, "a@example.com", domain.RegionBY)
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	h1, h2 := []byte("hash-1"), []byte("hash-2")
	if err := s.CreateSession(ctx, h1, uid, t0, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, h2, uid, t0, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, err := s.TouchSession(ctx, h1, t0.Add(30*time.Minute), t0.Add(2*time.Hour)); err != nil || got != uid {
		t.Fatalf("touch: %d %v", got, err)
	}
	// Продлённая сессия жива после исходного срока, непродлённая — нет.
	if _, err := s.TouchSession(ctx, h1, t0.Add(90*time.Minute), t0.Add(3*time.Hour)); err != nil {
		t.Fatalf("продлённая: %v", err)
	}
	if _, err := s.TouchSession(ctx, h2, t0.Add(90*time.Minute), t0.Add(3*time.Hour)); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("истёкшая: %v", err)
	}
	if err := s.DeleteSession(ctx, h1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TouchSession(ctx, h1, t0, t0.Add(time.Hour)); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("после выхода: %v", err)
	}
	n, err := s.PurgeExpired(ctx, t0.Add(2*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
	_ = s.CreateSession(ctx, h1, uid, t0, t0.Add(time.Hour))
	if err := s.DeleteUserSessions(ctx, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TouchSession(ctx, h1, t0, t0.Add(time.Hour)); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatal("DeleteUserSessions не завершил сессии")
	}
}

func TestLoginFailures(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 7; i++ {
		if err := s.RecordLoginFailure(ctx, "A@example.com", t0.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.RecentLoginFailures(ctx, "a@example.com", 5)
	if err != nil || len(got) != 5 || !got[0].Equal(t0.Add(6*time.Minute)) || !got[4].Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("%v %v", got, err)
	}
	if err := s.ClearLoginFailures(ctx, "a@example.com"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.RecentLoginFailures(ctx, "a@example.com", 5); len(got) != 0 {
		t.Fatalf("после очистки: %v", got)
	}
}

// REQ-FUNC-005: смена региона сбрасывает только условия «цена не выше X».
func TestUpdateProfileRegion(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	uid := newUser(t, s, "a@example.com", domain.RegionBY)
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	p := domain.Price{Availability: domain.Available, Final: 5999, Initial: 5999, Currency: "USD"}
	for _, app := range []domain.AppID{10, 20, 30} {
		if _, err := s.AddToWishlist(ctx, uid, domain.RegionBY, domain.Game{AppID: app, Name: "G"}, p, t0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE wishlist_items SET cond_type = CASE appid WHEN 10 THEN 'below' WHEN 20 THEN 'min_pct' ELSE 'any' END,
		       cond_value = CASE appid WHEN 10 THEN 3000 WHEN 20 THEN 50 END, last_notified_price = 5999`); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountConditions(ctx, uid, domain.ConditionBelow); n != 1 {
		t.Fatalf("below = %d", n)
	}
	pl, off := domain.RegionPL, false
	if err := s.UpdateProfile(ctx, uid, &pl, &off, nil); err != nil {
		t.Fatal(err)
	}
	prof, _ := s.Profile(ctx, uid)
	if prof.Region != domain.RegionPL || prof.EmailEnabled || !prof.TelegramEnabled {
		t.Fatalf("profile = %+v", prof)
	}
	rows, _ := s.pool.Query(ctx, `SELECT appid, cond_type, cond_value::int, last_notified_price FROM wishlist_items ORDER BY appid`)
	defer rows.Close()
	want := map[int]string{10: "any", 20: "min_pct", 30: "any"}
	for rows.Next() {
		var app int
		var cond string
		var val, last *int
		if err := rows.Scan(&app, &cond, &val, &last); err != nil {
			t.Fatal(err)
		}
		if cond != want[app] || last != nil || (app == 10 && val != nil) || (app == 20 && (val == nil || *val != 50)) {
			t.Errorf("appid %d: cond=%s val=%v last=%v", app, cond, val, last)
		}
	}
	// В новом регионе пары ещё не опрашивались: цена неизвестна.
	items, err := s.Wishlist(ctx, uid)
	if err != nil || len(items) != 3 || items[0].Price != nil || items[0].CheckedAt != nil || items[0].Region != domain.RegionPL {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

// REQ-FUNC-007: после удаления в БД нет записей пользователя, цены остаются.
func TestDeleteUser(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	uid := newUser(t, s, "a@example.com", domain.RegionBY)
	other := newUser(t, s, "b@example.com", domain.RegionBY)
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	p := domain.Price{Availability: domain.Available, Final: 5999, Initial: 5999, Currency: "USD"}
	g := domain.Game{AppID: 1245620, Name: "ELDEN RING"}
	if _, err := s.AddToWishlist(ctx, uid, domain.RegionBY, g, p, t0); err != nil {
		t.Fatal(err)
	}
	_ = s.CreateSession(ctx, []byte("h"), uid, t0, t0.Add(time.Hour))
	_ = s.RecordLoginFailure(ctx, "a@example.com", t0)
	_ = s.RecordLoginFailure(ctx, "b@example.com", t0)

	if err := s.DeleteUser(ctx, uid); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, uid); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("повторное удаление: %v", err)
	}
	var personal, prices int
	_ = s.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM users WHERE id = $1) + (SELECT count(*) FROM sessions WHERE user_id = $1) +
		(SELECT count(*) FROM wishlist_items WHERE user_id = $1) + (SELECT count(*) FROM login_failures WHERE email = 'a@example.com')`,
		uid).Scan(&personal)
	_ = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM price_state) + (SELECT count(*) FROM price_history)`).Scan(&prices)
	if personal != 0 || prices != 2 {
		t.Fatalf("personal=%d prices=%d", personal, prices)
	}
	if f, _ := s.RecentLoginFailures(ctx, "b@example.com", 5); len(f) != 1 {
		t.Fatal("удалены данные другого пользователя")
	}
	if _, err := s.Profile(ctx, other); err != nil {
		t.Fatal(err)
	}
}
