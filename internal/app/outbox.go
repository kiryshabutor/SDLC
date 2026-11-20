package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// Notifier — канал внешней доставки (DV-013): email (internal/mail) и с
// итерации 5 Telegram (internal/telegram).
type Notifier interface {
	Channel() string
	Send(ctx context.Context, msg domain.Message) error
}

// OutboxStore — операции очереди исходящих сообщений.
type OutboxStore interface {
	ClaimOutbox(ctx context.Context, channel string, now time.Time, limit int, lease time.Duration) ([]domain.OutboxItem, error)
	MarkSent(ctx context.Context, id int64, now time.Time) error
	MarkAttempt(ctx context.Context, id int64, attempts int, next time.Time, failed bool, errText string) error
}

// Параметры отправки (DV-013).
const (
	OutboxBatch    = 50
	OutboxInterval = 10 * time.Second
	outboxLease    = 2 * time.Minute
)

// Outbox отправляет записи email_outbox через Notifier. Порядок состояний
// записи — диаграмма SM-02: pending → sent | (повтор через 5 мин) | failed.
type Outbox struct {
	Store    OutboxStore
	Notifier Notifier
	Log      *slog.Logger
	Now      func() time.Time
}

// RunOnce отправляет одну пачку и возвращает число отправленных записей.
func (o *Outbox) RunOnce(ctx context.Context) (int, error) {
	items, err := o.Store.ClaimOutbox(ctx, o.Notifier.Channel(), o.now(), OutboxBatch, outboxLease)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, it := range items {
		if ctx.Err() != nil {
			return sent, ctx.Err()
		}
		err := o.Notifier.Send(ctx, it.Message)
		now := o.now()
		if err == nil {
			if err := o.Store.MarkSent(ctx, it.ID, now); err != nil {
				return sent, err
			}
			sent++
			o.Log.InfoContext(ctx, "message sent", "outbox_id", it.ID, "channel", it.Channel, "kind", it.Kind)
			continue
		}
		attempts, next, failed := domain.NextDelivery(it.Attempts, err, now)
		if err := o.Store.MarkAttempt(ctx, it.ID, attempts, next, failed, err.Error()); err != nil {
			return sent, err
		}
		level := slog.LevelWarn
		if failed {
			level = slog.LevelError
		}
		o.Log.Log(ctx, level, "message delivery failed", "outbox_id", it.ID, "channel", it.Channel,
			"kind", it.Kind, "attempts", attempts, "final", failed, "err", err)
	}
	return sent, nil
}

// Run отправляет очередь каждые OutboxInterval до отмены контекста.
func (o *Outbox) Run(ctx context.Context) {
	t := time.NewTicker(OutboxInterval)
	defer t.Stop()
	for {
		if _, err := o.RunOnce(ctx); err != nil && ctx.Err() == nil {
			o.Log.ErrorContext(ctx, "outbox run failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (o *Outbox) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}
