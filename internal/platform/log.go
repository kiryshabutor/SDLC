package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"strings"
)

type ctxKey struct{}

// NewLogger создаёт журнал JSON с полями component и version (REQ-OBS-001).
func NewLogger(w io.Writer, component, level string) *slog.Logger {
	var lvl slog.Level
	_ = lvl.UnmarshalText([]byte(level))
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	return slog.New(h).With("component", component, "version", Version)
}

// WithRequestID кладёт идентификатор запроса или цикла в контекст.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID возвращает идентификатор из контекста или пустую строку.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// NewRequestID возвращает случайный идентификатор из 16 hex-символов.
func NewRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// MaskEmail скрывает адрес в журналах: kirill@mail.com → k***@mail.com (REQ-OBS-004).
func MaskEmail(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}
