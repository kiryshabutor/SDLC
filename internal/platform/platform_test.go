package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
