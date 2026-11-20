package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

type outboxRow struct {
	domain.OutboxItem
	status string
	next   time.Time
}

type memOutbox struct{ rows []*outboxRow }

func (m *memOutbox) ClaimOutbox(_ context.Context, ch string, now time.Time, limit int, lease time.Duration) ([]domain.OutboxItem, error) {
	var out []domain.OutboxItem
	for _, r := range m.rows {
		if r.status == "pending" && r.Channel == ch && !r.next.After(now) && len(out) < limit {
			r.next = now.Add(lease)
			out = append(out, r.OutboxItem)
		}
	}
	return out, nil
}
func (m *memOutbox) MarkSent(_ context.Context, id int64, _ time.Time) error {
	m.rows[id-1].status = "sent"
	return nil
}
func (m *memOutbox) MarkAttempt(_ context.Context, id int64, attempts int, next time.Time, failed bool, _ string) error {
	r := m.rows[id-1]
	r.Attempts, r.next = attempts, next
	if failed {
		r.status = "failed"
	}
	return nil
}

type flakyNotifier struct {
	errs  []error // ошибки по очереди; затем успех
	sends int
}

func (f *flakyNotifier) Channel() string { return domain.ChannelEmail }
func (f *flakyNotifier) Send(context.Context, domain.Message) error {
	f.sends++
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		return err
	}
	return nil
}

func newOutbox(n *flakyNotifier, rows ...domain.Message) (*Outbox, *memOutbox, *clock) {
	st := &memOutbox{}
	c := &clock{time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)}
	for i, m := range rows {
		st.rows = append(st.rows, &outboxRow{OutboxItem: domain.OutboxItem{ID: int64(i + 1), Message: m}, status: "pending", next: c.t})
	}
	return &Outbox{Store: st, Notifier: n, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: c.now}, st, c
}

var mailMsg = domain.Message{Channel: domain.ChannelEmail, Kind: domain.KindVerify, Recipient: "a@example.com", Body: "x"}

// REQ-REL-005: SMTP недоступен — повтор через 5 минут, письмо уходит после
// восстановления в пределах 3 попыток.
func TestOutboxRetry(t *testing.T) {
	down := errors.New("connection refused")
	n := &flakyNotifier{errs: []error{down, down}}
	ob, st, c := newOutbox(n, mailMsg)
	ctx := context.Background()
	for i, wantSent := range []int{0, 0, 1} {
		sent, err := ob.RunOnce(ctx)
		if err != nil || sent != wantSent {
			t.Fatalf("попытка %d: sent=%d err=%v", i+1, sent, err)
		}
		if sent == 0 {
			if again, _ := ob.RunOnce(ctx); again != 0 {
				t.Fatal("повтор раньше 5 минут")
			}
			c.t = c.t.Add(domain.DeliveryRetryBackoff)
		}
	}
	if st.rows[0].status != "sent" || n.sends != 3 {
		t.Fatalf("status=%s sends=%d", st.rows[0].status, n.sends)
	}
}

func TestOutboxFailures(t *testing.T) {
	down := errors.New("timeout")
	n := &flakyNotifier{errs: []error{down, down, down}}
	ob, st, c := newOutbox(n, mailMsg)
	for i := 0; i < 3; i++ {
		_, _ = ob.RunOnce(context.Background())
		c.t = c.t.Add(domain.DeliveryRetryBackoff)
	}
	if st.rows[0].status != "failed" || st.rows[0].Attempts != 3 {
		t.Fatalf("после трёх неудач: %+v", st.rows[0])
	}

	// Окончательная ошибка — сразу failed.
	n = &flakyNotifier{errs: []error{errors.Join(domain.ErrPermanent, errors.New("550"))}}
	ob, st, _ = newOutbox(n, mailMsg)
	_, _ = ob.RunOnce(context.Background())
	if st.rows[0].status != "failed" || st.rows[0].Attempts != 1 {
		t.Fatalf("550: %+v", st.rows[0])
	}

	// retry_after переносит отправку и не считается попыткой.
	n = &flakyNotifier{errs: []error{&domain.RetryAfterError{After: 30 * time.Second, Err: errors.New("429")}}}
	ob, st, c = newOutbox(n, mailMsg)
	_, _ = ob.RunOnce(context.Background())
	if st.rows[0].Attempts != 0 || !st.rows[0].next.Equal(c.t.Add(30*time.Second)) || st.rows[0].status != "pending" {
		t.Fatalf("429: %+v", st.rows[0])
	}
}
