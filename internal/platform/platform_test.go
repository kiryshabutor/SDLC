package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{"kirill@mail.com": "k***@mail.com", "a@b.by": "a***@b.by", "broken": "***", "@x": "***"}
	for in, want := range cases {
		if got := MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestLoggerJSONFields(t *testing.T) {
	var buf bytes.Buffer
	NewLogger(&buf, "api", "info").Info("started", "request_id", "abc")
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"time", "level", "msg", "component", "version", "request_id"} {
		if _, ok := m[k]; !ok {
			t.Errorf("нет поля %q в %s", k, buf.String())
		}
	}
}

func TestWindow(t *testing.T) {
	now := time.Unix(0, 0)
	w := NewWindow(3, time.Minute)
	w.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if !w.Allow() {
			t.Fatalf("запрос %d отклонён", i+1)
		}
	}
	if w.Allow() {
		t.Fatal("четвёртый запрос в окне должен быть отклонён")
	}
	now = now.Add(61 * time.Second)
	if !w.Allow() {
		t.Fatal("после окна запрос должен проходить")
	}
	w.Pause(time.Minute)
	if w.Allow() {
		t.Fatal("во время паузы запросы запрещены")
	}
}

type tempErr struct{ temp bool }

func (e tempErr) Error() string   { return "e" }
func (e tempErr) Temporary() bool { return e.temp }

func TestRetry(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), 3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return tempErr{true}
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}

	calls = 0
	perm := tempErr{false}
	err = Retry(context.Background(), 3, time.Millisecond, func() error { calls++; return perm })
	if !errors.Is(err, perm) || calls != 1 {
		t.Fatalf("постоянная ошибка не повторяется: err=%v calls=%d", err, calls)
	}
}

// REQ-SEC-001: Argon2id с уникальной солью в формате PHC.
func TestPasswordHash(t *testing.T) {
	h1, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := HashPassword("secret123")
	if !strings.HasPrefix(h1, "$argon2id$v=19$m=19456,t=2,p=1$") || h1 == h2 {
		t.Fatalf("хэш %q, соль должна быть уникальной", h1)
	}
	if ok, err := CheckPassword(h1, "secret123"); !ok || err != nil {
		t.Fatalf("верный пароль: %v %v", ok, err)
	}
	if ok, _ := CheckPassword(h1, "secret124"); ok {
		t.Fatal("неверный пароль принят")
	}
	if ok, _ := CheckPassword(DummyPasswordHash, ""); ok {
		t.Fatal("пустой пароль совпал с фиктивным хэшем")
	}
	for _, bad := range []string{"", "plain", "$2a$12$bcrypt", "$argon2id$v=19$m=1,t=1,p=1$!!$!!", "$argon2id$v=18$m=1,t=1,p=1$YQ$YQ"} {
		if _, err := CheckPassword(bad, "x"); err == nil {
			t.Errorf("%q: ожидалась ошибка формата", bad)
		}
	}
}

// REQ-SEC-006: токен из 256 бит, в БД только SHA-256.
func TestTokens(t *testing.T) {
	a, b := NewToken(), NewToken()
	if len(a) != 43 || a == b {
		t.Fatalf("token %q", a)
	}
	if h := HashToken(a); len(h) != 32 || string(h) == a || !bytes.Equal(h, HashToken(a)) {
		t.Fatal("HashToken")
	}
}
