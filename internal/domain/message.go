package domain

import (
	"errors"
	"time"
)

// Каналы и виды исходящих сообщений (DV-013).
const (
	ChannelEmail    = "email"
	ChannelTelegram = "telegram"

	KindVerify = "verify"
	KindReset  = "reset"
	KindDigest = "digest"
)

// Назначения одноразовых токенов (auth_tokens.purpose).
const (
	PurposeVerify       = "verify"
	PurposeReset        = "reset"
	PurposeTelegramLink = "telegram_link"
)

// Сроки действия токенов и лимиты отправки писем.
const (
	VerifyTokenTTL       = 24 * time.Hour   // REQ-FUNC-002
	ResetTokenTTL        = time.Hour        // REQ-FUNC-004
	TelegramCodeTTL      = 15 * time.Minute // REQ-FUNC-031
	ResendInterval       = 60 * time.Second // REQ-FUNC-002
	MailRequestsPerHour  = 5                // REQ-SEC-005
	MaxDeliveryAttempts  = 3                // REQ-REL-005, DV-013
	DeliveryRetryBackoff = 5 * time.Minute
)

// Message — исходящее сообщение для записи в outbox.
type Message struct {
	UserID    int64
	Channel   string
	Kind      string
	Recipient string // email или chat_id
	Subject   string
	Body      string
}

// OutboxItem — запись очереди, выбранная для отправки.
type OutboxItem struct {
	ID       int64
	Attempts int
	Message
}

// RetryAfterError — канал просит повторить не раньше чем через After
// (Telegram 429 retry_after). Такая ошибка не считается неудачной попыткой.
type RetryAfterError struct {
	After time.Duration
	Err   error
}

func (e *RetryAfterError) Error() string {
	return "retry after " + e.After.String() + ": " + e.Err.Error()
}
func (e *RetryAfterError) Unwrap() error { return e.Err }

// ErrPermanent — повтор бессмыслен (адрес отклонён, бот заблокирован).
var ErrPermanent = errors.New("permanent delivery failure")

// NextDelivery решает судьбу записи после ошибки отправки: новое число
// попыток, время следующей попытки и признак окончательной неудачи.
func NextDelivery(attempts int, err error, now time.Time) (newAttempts int, next time.Time, failed bool) {
	var ra *RetryAfterError
	if errors.As(err, &ra) {
		return attempts, now.Add(ra.After), false
	}
	attempts++
	if errors.Is(err, ErrPermanent) || attempts >= MaxDeliveryAttempts {
		return attempts, now, true
	}
	return attempts, now.Add(DeliveryRetryBackoff), false
}

// TokenSpec — новый одноразовый токен для записи в auth_tokens.
type TokenSpec struct {
	Purpose string
	Hash    []byte
	Expires time.Time
}
