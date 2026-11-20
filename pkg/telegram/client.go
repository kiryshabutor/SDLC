// Package telegram — минимальный клиент Telegram Bot API: getUpdates
// (long polling) и sendMessage (REQ-INT-012). Не зависит от internal/*.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL — адрес Bot API.
const DefaultBaseURL = "https://api.telegram.org"

// Client обращается к боту с токеном Token.
type Client struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

// New создаёт клиент с таймаутом, превышающим время long polling.
func New(token string) *Client {
	return &Client{Token: token, BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: 70 * time.Second}}
}

// Update — входящее обновление (только сообщения).
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message — сообщение в чате с ботом.
type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

// Chat — чат; для личной переписки ID совпадает с ID пользователя.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// APIError — ответ Bot API с ok=false.
type APIError struct {
	Code        int
	Description string
	RetryAfter  time.Duration // для 429 Too Many Requests
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram: %d %s", e.Code, e.Description)
}

// IsForbidden — пользователь заблокировал бота (403).
func IsForbidden(err error) bool {
	var e *APIError
	return errors.As(err, &e) && e.Code == http.StatusForbidden
}

type response struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func (c *Client) call(ctx context.Context, method string, req, result any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/bot"+c.Token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return errors.New("telegram: bad request URL")
	}
	hr.Header.Set("Content-Type", "application/json")
	httpc := c.HTTP
	if httpc == nil {
		httpc = http.DefaultClient
	}
	resp, err := httpc.Do(hr)
	if err != nil {
		// url.Error содержит адрес с токеном бота: в журнал он попасть не должен.
		var ue *url.Error
		if errors.As(err, &ue) {
			return fmt.Errorf("telegram %s: %w", method, ue.Err)
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("telegram %s: HTTP %d: %w", method, resp.StatusCode, err)
	}
	if !r.OK {
		code := r.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return &APIError{Code: code, Description: r.Description, RetryAfter: time.Duration(r.Parameters.RetryAfter) * time.Second}
	}
	if result != nil {
		return json.Unmarshal(r.Result, result)
	}
	return nil
}

// GetUpdates ждёт новые сообщения до timeout секунд (long polling).
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	var out []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset": offset, "timeout": timeout, "allowed_updates": []string{"message"},
	}, &out)
	return out, err
}

// SendMessage отправляет текст в чат.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID, "text": text, "disable_web_page_preview": true,
	}, nil)
}
