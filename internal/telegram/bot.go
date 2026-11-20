// Package telegram — бот сервиса: привязка аккаунта по одноразовому коду и
// команда /stop (REQ-FUNC-031, DV-013, ADR-0009). Отправка дайджестов через
// outbox (TelegramNotifier) добавляется в итерации 5.
package telegram

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/i18n"
	tg "github.com/kiryshabutor/SDLC/pkg/telegram"
)

// API — методы Bot API, нужные боту (реализация: pkg/telegram.Client).
type API interface {
	GetUpdates(ctx context.Context, offset int64, timeout int) ([]tg.Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// Store — привязка и отвязка чатов.
type Store interface {
	LinkTelegram(ctx context.Context, hash []byte, chatID int64, now time.Time) (int64, error)
	UnlinkTelegramChat(ctx context.Context, chatID int64) (bool, error)
}

// Bot обрабатывает сообщения боту.
type Bot struct {
	API       API
	Store     Store
	HashToken func(string) []byte
	Texts     *i18n.Catalog
	Log       *slog.Logger
	Now       func() time.Time
	// PollTimeout — время ожидания getUpdates в секундах (по умолчанию 30).
	PollTimeout int
}

// codeRe — формат кода привязки: 256 бит в base64url (platform.NewToken).
var codeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// Reply возвращает ответ на сообщение или пустую строку, если отвечать не
// нужно. Код принимается как параметр /start (ссылка t.me/<бот>?start=<код>)
// и как отдельное сообщение, если пользователь вставил его вручную.
func (b *Bot) Reply(ctx context.Context, m *tg.Message) (string, error) {
	if m == nil || m.Chat.Type != "private" {
		return "", nil
	}
	text := strings.TrimSpace(m.Text)
	cmd, arg, _ := strings.Cut(text, " ")
	cmd = strings.SplitN(cmd, "@", 2)[0] // /start@my_bot в некоторых клиентах
	arg = strings.TrimSpace(arg)

	switch {
	case cmd == "/stop":
		ok, err := b.Store.UnlinkTelegramChat(ctx, m.Chat.ID)
		if err != nil {
			return "", err
		}
		if !ok {
			return b.Texts.T("telegram.not_linked"), nil
		}
		b.Log.InfoContext(ctx, "telegram unlinked by /stop", "chat_id", m.Chat.ID)
		return b.Texts.T("telegram.stopped"), nil
	case cmd == "/start" && arg != "":
		return b.link(ctx, m.Chat.ID, arg)
	case codeRe.MatchString(text):
		return b.link(ctx, m.Chat.ID, text)
	default:
		return b.Texts.T("telegram.help"), nil
	}
}

func (b *Bot) link(ctx context.Context, chatID int64, code string) (string, error) {
	uid, err := b.Store.LinkTelegram(ctx, b.HashToken(code), chatID, b.now())
	if errors.Is(err, domain.ErrTokenInvalid) {
		return b.Texts.T("telegram.code_invalid"), nil
	}
	if err != nil {
		return "", err
	}
	b.Log.InfoContext(ctx, "telegram linked", "user_id", uid, "chat_id", chatID)
	return b.Texts.T("telegram.linked"), nil
}

// Run получает обновления, пока не отменён контекст. Ошибки сети и Bot API
// не останавливают бота: следующая попытка через 5 секунд (или через
// retry_after при 429).
func (b *Bot) Run(ctx context.Context) {
	timeout := b.PollTimeout
	if timeout == 0 {
		timeout = 30
	}
	var offset int64
	for ctx.Err() == nil {
		updates, err := b.API.GetUpdates(ctx, offset, timeout)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.Log.WarnContext(ctx, "telegram getUpdates failed", "err", err)
			b.sleep(ctx, retryDelay(err))
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			reply, err := b.Reply(ctx, u.Message)
			if err != nil {
				b.Log.ErrorContext(ctx, "telegram update failed", "update_id", u.UpdateID, "err", err)
				continue
			}
			if reply == "" {
				continue
			}
			if err := b.send(ctx, u.Message.Chat.ID, reply); err != nil {
				b.Log.WarnContext(ctx, "telegram reply failed", "chat_id", u.Message.Chat.ID, "err", err)
			}
		}
	}
}

// send отвечает в чат и один раз повторяет после паузы retry_after (429).
func (b *Bot) send(ctx context.Context, chatID int64, text string) error {
	err := b.API.SendMessage(ctx, chatID, text)
	var ae *tg.APIError
	if errors.As(err, &ae) && ae.RetryAfter > 0 {
		b.sleep(ctx, ae.RetryAfter)
		err = b.API.SendMessage(ctx, chatID, text)
	}
	return err
}

func retryDelay(err error) time.Duration {
	var ae *tg.APIError
	if errors.As(err, &ae) && ae.RetryAfter > 0 {
		return ae.RetryAfter
	}
	return 5 * time.Second
}

func (b *Bot) sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func (b *Bot) now() time.Time {
	if b.Now != nil {
		return b.Now().UTC()
	}
	return time.Now().UTC()
}
