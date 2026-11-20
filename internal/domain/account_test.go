package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeEmail(t *testing.T) {
	ok := map[string]string{
		"kirill@mail.com":          "kirill@mail.com",
		"  Kirill.Butor@BSUIR.by ": "kirill.butor@bsuir.by",
		"a+tag@sub.example.org":    "a+tag@sub.example.org",
	}
	for in, want := range ok {
		if got, err := NormalizeEmail(in); err != nil || got != want {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "kirill", "kirill@", "@mail.com", "kirill@localhost", "Иван <ivan@mail.com>",
		"a b@mail.com", strings.Repeat("a", 250) + "@m.by"} {
		if _, err := NormalizeEmail(in); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("%q: ожидалась ErrInvalidEmail, got %v", in, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	for _, p := range []string{"secret12", "пароль123", "Abcdefg1"} {
		if err := ValidatePassword(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
	for _, p := range []string{"", "abc1", "abcdefgh", "12345678", "пароль12"[:7], strings.Repeat("a1", 65)} {
		if err := ValidatePassword(p); !errors.Is(err, ErrWeakPassword) {
			t.Errorf("%q: ожидалась ErrWeakPassword", p)
		}
	}
}

func TestRegistrationValidate(t *testing.T) {
	good := Registration{Email: "A@B.by", Password: "secret123", PasswordConfirm: "secret123", Region: "PL", Consent: true}
	email, region, err := good.Validate()
	if err != nil || email != "a@b.by" || region != RegionPL {
		t.Fatalf("%q %q %v", email, region, err)
	}
	cases := []struct {
		mut  func(*Registration)
		want error
	}{
		{func(r *Registration) { r.Email = "bad" }, ErrInvalidEmail},
		{func(r *Registration) { r.Password, r.PasswordConfirm = "short", "short" }, ErrWeakPassword},
		{func(r *Registration) { r.PasswordConfirm = "secret124" }, ErrPasswordMismatch},
		{func(r *Registration) { r.Region = "DE" }, ErrUnknownRegion},
		{func(r *Registration) { r.Consent = false }, ErrConsentRequired},
		// Порядок полей формы: первой сообщается ошибка email.
		{func(r *Registration) { r.Email, r.Consent = "bad", false }, ErrInvalidEmail},
	}
	for i, tc := range cases {
		r := good
		tc.mut(&r)
		if _, _, err := r.Validate(); !errors.Is(err, tc.want) {
			t.Errorf("case %d: got %v, want %v", i, err, tc.want)
		}
	}
}

func TestLockedUntil(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	// series возвращает n попыток с шагом step, от новых к старым.
	series := func(n int, step time.Duration) []time.Time {
		out := make([]time.Time, n)
		for i := range out {
			out[i] = t0.Add(-time.Duration(i) * step)
		}
		return out
	}
	if _, locked := LockedUntil(series(4, time.Minute), t0); locked {
		t.Fatal("4 попытки не блокируют")
	}
	until, locked := LockedUntil(series(5, time.Minute), t0.Add(time.Second))
	if !locked || !until.Equal(t0.Add(15*time.Minute)) {
		t.Fatalf("5 попыток за 4 минуты: until=%v locked=%v", until, locked)
	}
	if _, locked := LockedUntil(series(5, time.Minute), t0.Add(15*time.Minute)); locked {
		t.Fatal("блокировка длится 15 минут")
	}
	if _, locked := LockedUntil(series(5, 4*time.Minute), t0); locked {
		t.Fatal("5 попыток за 16 минут не укладываются в окно")
	}
	if _, locked := LockedUntil(series(5, 3*time.Minute+45*time.Second), t0); !locked {
		t.Fatal("5 попыток ровно за 15 минут блокируют")
	}
}
