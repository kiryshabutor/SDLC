package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetUpdatesAndSend(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/botTOKEN/getUpdates":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req["offset"] != 5.0 || req["timeout"] != 30.0 {
				t.Errorf("getUpdates req = %v", req)
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":5,"message":{"message_id":1,"chat":{"id":42,"type":"private"},"text":"/start abc"}}]}`))
		case "/botTOKEN/sendMessage":
			_ = json.NewDecoder(r.Body).Decode(&sent)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{Token: "TOKEN", BaseURL: srv.URL, HTTP: srv.Client()}
	ups, err := c.GetUpdates(context.Background(), 5, 30)
	if err != nil || len(ups) != 1 || ups[0].Message.Chat.ID != 42 || ups[0].Message.Text != "/start abc" {
		t.Fatalf("%+v %v", ups, err)
	}
	if err := c.SendMessage(context.Background(), 42, "Привет"); err != nil || sent["chat_id"] != 42.0 || sent["text"] != "Привет" {
		t.Fatalf("sent=%v err=%v", sent, err)
	}
}

// REQ-INT-012: 429 возвращает retry_after, 403 — признак блокировки бота.
func TestAPIErrors(t *testing.T) {
	status := 429
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		if status == 429 {
			_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 7","parameters":{"retry_after":7}}`))
		} else {
			_, _ = w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))
		}
	}))
	defer srv.Close()
	c := &Client{Token: "SECRET", BaseURL: srv.URL, HTTP: srv.Client()}
	err := c.SendMessage(context.Background(), 1, "x")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != 429 || ae.RetryAfter != 7*time.Second || IsForbidden(err) {
		t.Fatalf("429: %v", err)
	}
	status = 403
	if err := c.SendMessage(context.Background(), 1, "x"); !IsForbidden(err) {
		t.Fatalf("403: %v", err)
	}
}

// Токен бота не попадает в текст сетевой ошибки.
func TestTokenNotInErrors(t *testing.T) {
	c := &Client{Token: "SECRET-TOKEN", BaseURL: "http://127.0.0.1:1", HTTP: &http.Client{Timeout: time.Second}}
	err := c.SendMessage(context.Background(), 1, "x")
	if err == nil || strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Fatalf("err = %v", err)
	}
}
