// Package steamstore — клиент недокументированного Steam Store API
// (storesearch, appdetails). Пакет не зависит от бизнес-логики Системы
// и может быть вынесен в отдельный модуль (REQ-REUSE-001, DV-003).
package steamstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL — адрес Steam Store API.
const DefaultBaseURL = "https://store.steampowered.com"

// ErrNotFound возвращается, когда Steam отвечает success=false для appid.
var ErrNotFound = errors.New("steamstore: app not found")

// StatusError — ответ Steam с кодом, отличным от 200.
type StatusError struct {
	Code       int
	RetryAfter time.Duration
}

func (e *StatusError) Error() string { return fmt.Sprintf("steamstore: HTTP %d", e.Code) }

// Temporary сообщает, имеет ли смысл повторить запрос (5xx, 429, 403 при лимите).
func (e *StatusError) Temporary() bool {
	return e.Code >= 500 || e.Code == http.StatusTooManyRequests || e.Code == http.StatusForbidden
}

// Client выполняет запросы к Steam Store API.
type Client struct {
	BaseURL   string
	HTTP      *http.Client
	Language  string
	UserAgent string
}

// New создаёт клиент с таймаутом 10 с (REQ-REL-001).
func New() *Client {
	return &Client{
		BaseURL:   DefaultBaseURL,
		HTTP:      &http.Client{Timeout: 10 * time.Second},
		Language:  "russian",
		UserAgent: "steam-deal-hunter/0.1",
	}
}

// Price — блок price_overview в минимальных единицах валюты.
type Price struct {
	Currency        string `json:"currency"`
	Initial         int    `json:"initial"`
	Final           int    `json:"final"`
	DiscountPercent int    `json:"discount_percent"`
}

// App — нужная Системе часть ответа appdetails.
type App struct {
	AppID       int    `json:"steam_appid"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	IsFree      bool   `json:"is_free"`
	HeaderImage string `json:"header_image"`
	Price       *Price `json:"price_overview"`
	ReleaseDate struct {
		ComingSoon bool   `json:"coming_soon"`
		Date       string `json:"date"`
	} `json:"release_date"`
}

// SearchItem — элемент ответа storesearch.
type SearchItem struct {
	Type      string `json:"type"`
	ID        int    `json:"id"`
	Name      string `json:"name"`
	TinyImage string `json:"tiny_image"`
	Price     *struct {
		Currency string `json:"currency"`
		Initial  int    `json:"initial"`
		Final    int    `json:"final"`
	} `json:"price"`
}

// AppDetails возвращает сведения об играх в регионе cc. Ключ результата —
// appid; игры, для которых Steam вернул success=false, в результат не входят.
// Steam принимает несколько appid только вместе с filters=price_overview,
// поэтому для одной игры запрашиваются полные сведения, а для пакета — цены.
func (c *Client) AppDetails(ctx context.Context, cc string, appIDs ...int) (map[int]App, error) {
	if len(appIDs) == 0 {
		return map[int]App{}, nil
	}
	ids := make([]string, len(appIDs))
	for i, id := range appIDs {
		ids[i] = strconv.Itoa(id)
	}
	q := url.Values{"appids": {strings.Join(ids, ",")}, "cc": {cc}, "l": {c.Language}}
	if len(appIDs) > 1 {
		q.Set("filters", "price_overview")
	}
	var raw map[string]struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.get(ctx, "/api/appdetails", q, &raw); err != nil {
		return nil, err
	}
	out := make(map[int]App, len(raw))
	for key, v := range raw {
		id, err := strconv.Atoi(key)
		if err != nil || !v.Success {
			continue
		}
		var app App
		// При пустой цене Steam возвращает data как [], а не объект.
		if len(v.Data) > 0 && v.Data[0] == '{' {
			if err := json.Unmarshal(v.Data, &app); err != nil {
				return nil, fmt.Errorf("steamstore: decode app %d: %w", id, err)
			}
		}
		app.AppID = id
		out[id] = app
	}
	return out, nil
}

// App возвращает одну игру или ErrNotFound.
func (c *Client) App(ctx context.Context, cc string, appID int) (App, error) {
	apps, err := c.AppDetails(ctx, cc, appID)
	if err != nil {
		return App{}, err
	}
	app, ok := apps[appID]
	if !ok {
		return App{}, ErrNotFound
	}
	return app, nil
}

// Search ищет игры по названию в регионе cc.
func (c *Client) Search(ctx context.Context, cc, term string) ([]SearchItem, error) {
	q := url.Values{"term": {term}, "cc": {cc}, "l": {c.Language}}
	var resp struct {
		Items []SearchItem `json:"items"`
	}
	if err := c.get(ctx, "/api/storesearch/", q, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return &StatusError{Code: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	// Steam иногда отвечает "null" на несуществующий appid.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(body)) == "null" {
		return ErrNotFound
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("steamstore: decode %s: %w", path, err)
	}
	return nil
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(v); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// RetryAfterHint возвращает значение заголовка Retry-After.
func (e *StatusError) RetryAfterHint() time.Duration { return e.RetryAfter }
