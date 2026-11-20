package telegram

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/i18n"
	tg "github.com/kiryshabutor/SDLC/pkg/telegram"
)

const validCode = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJ0123456"

type fakeStore struct {
	mu     sync.Mutex
	codes  map[string]int64 // хэш → user_id
	linked map[int64]int64  // chat → user
}

func (f *fakeStore) LinkTelegram(_ context.Context, hash []byte, chat int64, _ time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	uid, ok := f.codes[string(hash)]
	if !ok {
		return 0, domain.ErrTokenInvalid
	}
	delete(f.codes, string(hash)) // одноразовый
	f.linked[chat] = uid
	return uid, nil
}

func (f *fakeStore) UnlinkTelegramChat(_ context.Context, chat int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.linked[chat]
	delete(f.linked, chat)
	return ok, nil
}

func newBot(api API) (*Bot, *fakeStore) {
	st := &fakeStore{codes: map[string]int64{"h:" + validCode: 7}, linked: map[int64]int64{}}
	return &Bot{API: api, Store: st, HashToken: func(s string) []byte { return []byte("h:" + s) },
		Texts: i18n.MustLoad(i18n.DefaultLang), Log: slog.New(slog.NewTextHandler(io.Discard, nil)), PollTimeout: 1}, st
}

func msg(chat int64, text string) *tg.Message {
	return &tg.Message{Chat: tg.Chat{ID: chat, Type: "private"}, Text: text}
}

func TestReply(t *testing.T) {
	b, st := newBot(nil)
	ctx := context.Background()
	texts := i18n.MustLoad(i18n.DefaultLang)

	cases := []struct {
		m    *tg.Message
		want string
	}{
		{msg(42, "/start"), texts.T("telegram.help")},
		{msg(42, "привет"), texts.T("telegram.help")},
		{msg(42, "/stop"), texts.T("telegram.not_linked")},
		{msg(42, "/start "+validCode), texts.T("telegram.linked")},
		{msg(42, "/start "+validCode), texts.T("telegram.code_invalid")}, // повторно
		{msg(42, "/stop@deal_bot"), texts.T("telegram.stopped")},
		{&tg.Message{Chat: tg.Chat{ID: -1, Type: "group"}, Text: "/stop"}, ""},
		{nil, ""},
	}
	for i, tc := range cases {
		got, err := b.Reply(ctx, tc.m)
		if err != nil || got != tc.want {
			t.Errorf("case %d: got %q, err %v", i, got, err)
		}
		if i == 3 && st.linked[42] != 7 {
			t.Fatal("чат не привязан")
		}
	}
	// Код, вставленный отдельным сообщением.
	st.codes["h:"+validCode] = 9
	if got, _ := b.Reply(ctx, msg(43, validCode)); got != texts.T("telegram.linked") || st.linked[43] != 9 {
		t.Fatalf("код без /start: %q", got)
	}
}

// fakeAPI отдаёт одно обновление, затем 429 на первый ответ и пустые ленты.
type fakeAPI struct {
	mu      sync.Mutex
	calls   int
	sends   []string
	offsets []int64
	retried bool
}

func (f *fakeAPI) GetUpdates(ctx context.Context, offset int64, _ int) ([]tg.Update, error) {
	f.mu.Lock()
	f.calls++
	f.offsets = append(f.offsets, offset)
	n := f.calls
	f.mu.Unlock()
	if n == 1 {
		return []tg.Update{{UpdateID: 100, Message: msg(42, "/start "+validCode)}}, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeAPI) SendMessage(_ context.Context, _ int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.retried {
		f.retried = true
		return &tg.APIError{Code: 429, RetryAfter: 10 * time.Millisecond}
	}
	f.sends = append(f.sends, text)
	return nil
}

func TestRunLinksAndRetries429(t *testing.T) {
	api := &fakeAPI{}
	b, st := newBot(api)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()
	deadline := time.After(2 * time.Second)
	for {
		api.mu.Lock()
		n := len(api.sends)
		api.mu.Unlock()
		if n == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("ответ не отправлен")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done
	if st.linked[42] != 7 || !strings.HasPrefix(api.sends[0], "Готово!") {
		t.Fatalf("linked=%v sends=%v", st.linked, api.sends)
	}
	if len(api.offsets) < 2 || api.offsets[1] != 101 {
		t.Fatalf("offset должен сдвигаться: %v", api.offsets)
	}
}
