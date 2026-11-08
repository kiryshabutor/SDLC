package platform

import (
	"context"
	"errors"
	"time"
)

// Retryable — ошибка, которую стоит повторить, с подсказкой задержки.
type Retryable interface {
	Temporary() bool
}

type retryAfter interface{ RetryAfterHint() time.Duration }

// Retry выполняет fn до attempts раз с задержками base, 2·base, 4·base…
// Повторяются только временные ошибки и ошибки сети (REQ-REL-001).
func Retry(ctx context.Context, attempts int, base time.Duration, fn func() error) error {
	var err error
	delay := base
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil || !temporary(err) || i == attempts-1 {
			return err
		}
		wait := delay
		var ra retryAfter
		if errors.As(err, &ra) && ra.RetryAfterHint() > wait {
			wait = ra.RetryAfterHint()
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		delay *= 2
	}
	return err
}

func temporary(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var r Retryable
	if errors.As(err, &r) {
		return r.Temporary()
	}
	// Ошибки транспорта (таймаут, обрыв соединения) считаются временными.
	return errors.Is(err, context.DeadlineExceeded) || isNetErr(err)
}
