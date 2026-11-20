package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

var t0 = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func verifyMail(email string) *domain.Message {
	return &domain.Message{Channel: domain.ChannelEmail, Kind: domain.KindVerify, Recipient: email, Subject: "s", Body: "b"}
}

// DV-008: пользователь, токен и письмо создаются в одной транзакции.
func TestCreateUserWithVerification(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	tok := &domain.TokenSpec{Purpose: domain.PurposeVerify, Hash: []byte("v1"), Expires: t0.Add(24 * time.Hour)}
	uid, err := s.CreateUser(ctx, "a@example.com", "h", domain.RegionBY, t0, tok, verifyMail("a@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	// Занятый email откатывает всю транзакцию: ни токена, ни письма.
	tok2 := &domain.TokenSpec{Purpose: domain.PurposeVerify, Hash: []byte("v2"), Expires: t0.Add(time.Hour)}
	if _, err := s.CreateUser(ctx, "A@example.com", "h", domain.RegionBY, t0, tok2, verifyMail("A@example.com")); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("дубликат: %v", err)
	}
	var tokens, mails int
	_ = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM auth_tokens), (SELECT count(*) FROM email_outbox)`).Scan(&tokens, &mails)
	if tokens != 1 || mails != 1 {
		t.Fatalf("tokens=%d mails=%d", tokens, mails)
	}

	if _, err := s.VerifyEmail(ctx, []byte("v1"), t0.Add(25*time.Hour)); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("истёкший: %v", err)
	}
	if got, err := s.VerifyEmail(ctx, []byte("v1"), t0.Add(time.Hour)); err != nil || got != uid {
		t.Fatalf("verify: %d %v", got, err)
	}
	if _, err := s.VerifyEmail(ctx, []byte("v1"), t0.Add(time.Hour)); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("повторно: %v", err)
	}
	if p, _ := s.Profile(ctx, uid); !p.EmailVerified {
		t.Fatal("email_verified_at не заполнен")
	}
	// Токен другого назначения не подходит.
	_ = s.IssueToken(ctx, uid, domain.TokenSpec{Purpose: domain.PurposeReset, Hash: []byte("r1"), Expires: t0.Add(time.Hour)}, nil, t0)
	if _, err := s.VerifyEmail(ctx, []byte("r1"), t0); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("reset вместо verify: %v", err)
	}
}

func TestIssueTokenAndStats(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	uid := newUser(t, s, "a@example.com", domain.RegionBY)
	for i, h := range []string{"r1", "r2", "r3"} {
		at := t0.Add(time.Duration(i) * 20 * time.Minute)
		if err := s.IssueToken(ctx, uid, domain.TokenSpec{Purpose: domain.PurposeReset, Hash: []byte(h), Expires: at.Add(time.Hour)}, nil, at); err != nil {
			t.Fatal(err)
		}
	}
	last, n, err := s.TokenStats(ctx, uid, domain.PurposeReset, t0.Add(30*time.Minute))
	if err != nil || n != 1 || last == nil || !last.Equal(t0.Add(40*time.Minute)) {
		t.Fatalf("last=%v n=%d err=%v", last, n, err)
	}
	// Действует только последний токен назначения.
	if _, err := s.ResetPassword(ctx, []byte("r1"), "new", t0.Add(45*time.Minute)); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("старый токен: %v", err)
	}
	_ = s.CreateSession(ctx, []byte("sess"), uid, t0, t0.Add(time.Hour))
	_ = s.RecordLoginFailure(ctx, "a@example.com", t0)
	if _, err := s.ResetPassword(ctx, []byte("r3"), "new-hash", t0.Add(45*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if h, _ := s.PasswordHash(ctx, uid); h != "new-hash" {
		t.Fatalf("hash = %q", h)
	}
	if _, err := s.TouchSession(ctx, []byte("sess"), t0, t0.Add(time.Hour)); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatal("сессии должны быть завершены (REQ-FUNC-004)")
	}
	if f, _ := s.RecentLoginFailures(ctx, "a@example.com", 5); len(f) != 0 {
		t.Fatal("счётчик попыток не очищен")
	}
}

func TestTelegramLinking(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	a := newUser(t, s, "a@example.com", domain.RegionBY)
	b := newUser(t, s, "b@example.com", domain.RegionBY)
	issue := func(uid int64, h string) {
		_ = s.IssueToken(ctx, uid, domain.TokenSpec{Purpose: domain.PurposeTelegramLink, Hash: []byte(h), Expires: t0.Add(15 * time.Minute)}, nil, t0)
	}
	issue(a, "ca")
	if _, err := s.LinkTelegram(ctx, []byte("ca"), 4242, t0.Add(16*time.Minute)); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("просроченный код: %v", err)
	}
	issue(a, "ca2")
	if uid, err := s.LinkTelegram(ctx, []byte("ca2"), 4242, t0.Add(time.Minute)); err != nil || uid != a {
		t.Fatalf("link: %d %v", uid, err)
	}
	if p, _ := s.Profile(ctx, a); !p.TelegramLinked {
		t.Fatal("не привязан")
	}
	// Тот же чат привязывается к другой учётной записи и отвязывается от первой.
	issue(b, "cb")
	if _, err := s.LinkTelegram(ctx, []byte("cb"), 4242, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	pa, _ := s.Profile(ctx, a)
	pb, _ := s.Profile(ctx, b)
	if pa.TelegramLinked || !pb.TelegramLinked {
		t.Fatalf("a=%v b=%v", pa.TelegramLinked, pb.TelegramLinked)
	}
	if ok, err := s.UnlinkTelegramChat(ctx, 4242); !ok || err != nil {
		t.Fatalf("/stop: %v %v", ok, err)
	}
	if ok, _ := s.UnlinkTelegramChat(ctx, 4242); ok {
		t.Fatal("повторный /stop")
	}
}

// SM-02: pending → повтор → sent | failed; захват не отдаёт запись дважды.
func TestOutbox(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	uid := newUser(t, s, "a@example.com", domain.RegionBY)
	for i := 0; i < 3; i++ {
		tok := domain.TokenSpec{Purpose: domain.PurposeReset, Hash: []byte{byte(i)}, Expires: t0.Add(time.Hour)}
		if err := s.IssueToken(ctx, uid, tok, verifyMail("a@example.com"), t0); err != nil {
			t.Fatal(err)
		}
	}
	items, err := s.ClaimOutbox(ctx, domain.ChannelEmail, t0, 2, time.Minute)
	if err != nil || len(items) != 2 || items[0].Recipient != "a@example.com" || items[0].UserID != uid {
		t.Fatalf("claim: %+v %v", items, err)
	}
	rest, _ := s.ClaimOutbox(ctx, domain.ChannelEmail, t0, 10, time.Minute)
	if len(rest) != 1 {
		t.Fatalf("захваченные записи не должны выдаваться повторно: %d", len(rest))
	}
	if tg, _ := s.ClaimOutbox(ctx, domain.ChannelTelegram, t0.Add(time.Hour), 10, time.Minute); len(tg) != 0 {
		t.Fatal("чужой канал")
	}
	_ = s.MarkSent(ctx, items[0].ID, t0)
	_ = s.MarkAttempt(ctx, items[1].ID, 1, t0.Add(5*time.Minute), false, "timeout")
	_ = s.MarkAttempt(ctx, rest[0].ID, 3, t0, true, "timeout")
	again, _ := s.ClaimOutbox(ctx, domain.ChannelEmail, t0.Add(5*time.Minute), 10, time.Minute)
	if len(again) != 1 || again[0].ID != items[1].ID || again[0].Attempts != 1 {
		t.Fatalf("повтор: %+v", again)
	}
	var statuses string
	_ = s.pool.QueryRow(ctx, `SELECT string_agg(status, ',' ORDER BY id) FROM email_outbox`).Scan(&statuses)
	if statuses != "sent,pending,failed" {
		t.Fatalf("statuses = %s", statuses)
	}
	// Удаление пользователя удаляет его письма и токены (REQ-FUNC-007).
	_ = s.DeleteUser(ctx, uid)
	var n int
	_ = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM email_outbox) + (SELECT count(*) FROM auth_tokens)`).Scan(&n)
	if n != 0 {
		t.Fatalf("осталось %d записей", n)
	}
}

func TestRemoveAndCondition(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	uid := newUser(t, s, "a@example.com", domain.RegionBY)
	p := domain.Price{Availability: domain.Available, Final: 4999, Initial: 9999, DiscountPct: 50, Currency: "USD"}
	if _, err := s.AddToWishlist(ctx, uid, domain.RegionBY, domain.Game{AppID: 10, Name: "G"}, p, t0); err != nil {
		t.Fatal(err)
	}
	_, _ = s.pool.Exec(ctx, `UPDATE wishlist_items SET last_notified_price = 4999`)
	below := domain.ItemCondition{Type: domain.ConditionBelow, Value: 3000}
	if err := s.SetCondition(ctx, uid, 10, below); err != nil {
		t.Fatal(err)
	}
	items, _ := s.Wishlist(ctx, uid)
	if items[0].Condition != below || items[0].HistLow == nil || *items[0].HistLow != 4999 || !items[0].AtHistLow {
		t.Fatalf("item = %+v", items[0])
	}
	var last *int
	_ = s.pool.QueryRow(ctx, `SELECT last_notified_price FROM wishlist_items`).Scan(&last)
	if last != nil {
		t.Fatal("смена условия должна сбросить last_notified_price (UC-07)")
	}
	// То же условие повторно не сбрасывает last_notified_price.
	_, _ = s.pool.Exec(ctx, `UPDATE wishlist_items SET last_notified_price = 2500`)
	_ = s.SetCondition(ctx, uid, 10, below)
	_ = s.pool.QueryRow(ctx, `SELECT last_notified_price FROM wishlist_items`).Scan(&last)
	if last == nil || *last != 2500 {
		t.Fatalf("last = %v", last)
	}
	if err := s.SetCondition(ctx, uid, 999, below); !errors.Is(err, domain.ErrNotInWishlist) {
		t.Fatalf("нет игры: %v", err)
	}
	if err := s.RemoveFromWishlist(ctx, uid, 10); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveFromWishlist(ctx, uid, 10); !errors.Is(err, domain.ErrNotInWishlist) {
		t.Fatalf("повторно: %v", err)
	}
	var prices int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM price_state`).Scan(&prices)
	if prices != 1 {
		t.Fatal("цены игры удалять нельзя")
	}
}
