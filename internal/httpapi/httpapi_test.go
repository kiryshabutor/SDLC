package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

type fakeWishlist struct{ err error }

func (f fakeWishlist) Find(context.Context, int64, string) ([]domain.GameHit, error) {
	return nil, f.err
}
func (f fakeWishlist) List(context.Context, int64) ([]domain.WishlistItem, error) { return nil, f.err }
func (f fakeWishlist) Add(_ context.Context, _ int64, ref string) (domain.WishlistItem, error) {
	if f.err != nil {
		return domain.WishlistItem{}, f.err
	}
	return domain.WishlistItem{Game: domain.Game{AppID: 1245620, Name: "ELDEN RING"}, Condition: domain.ConditionAny}, nil
}

type fakeHealth struct {
	pingErr error
	last    *time.Time
}

func (f fakeHealth) Ping(context.Context) error                              { return f.pingErr }
func (f fakeHealth) LastCycleFinishedAt(context.Context) (*time.Time, error) { return f.last, nil }

func newServer(w WishlistService, h Health) http.Handler {
	s := &Server{Wishlist: w, Health: h, Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		UserID: func(*http.Request) (int64, bool) { return 1, true }}
	return s.Handler()
}

func do(t *testing.T, h http.Handler, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec, m
}

func TestHealthz(t *testing.T) {
	last := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	rec, m := do(t, newServer(fakeWishlist{}, fakeHealth{last: &last}), "GET", "/healthz", "")
	if rec.Code != 200 || m["status"] != "ok" || m["last_cycle_at"] != "2026-11-08T09:00:00Z" || m["version"] == nil {
		t.Fatalf("code=%d body=%v", rec.Code, m)
	}
	rec, m = do(t, newServer(fakeWishlist{}, fakeHealth{pingErr: errors.New("down")}), "GET", "/healthz", "")
	if rec.Code != 503 || m["status"] != "db_unavailable" {
		t.Fatalf("code=%d body=%v", rec.Code, m)
	}
}

func TestAddToWishlist(t *testing.T) {
	rec, m := do(t, newServer(fakeWishlist{}, fakeHealth{}), "POST", "/api/v1/wishlist", `{"ref":"1245620"}`)
	if rec.Code != 201 || m["name"] != "ELDEN RING" || m["condition"] != "any" {
		t.Fatalf("code=%d body=%v", rec.Code, m)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("нет X-Request-ID")
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
		key  string
	}{
		{domain.ErrInvalidAppRef, 400, "invalid_app_ref"},
		{domain.ErrAlreadyInWishlist, 409, "already_in_wishlist"},
		{domain.ErrWishlistFull, 422, "wishlist_full"},
		{domain.ErrGameNotFound, 404, "game_not_found"},
		{errors.Join(domain.ErrSourceUnavailable, errors.New("502")), 503, "steam_unavailable"},
		{errors.New("boom"), 500, "internal"},
	}
	for _, tc := range cases {
		rec, m := do(t, newServer(fakeWishlist{err: tc.err}, fakeHealth{}), "POST", "/api/v1/wishlist", `{"ref":"x"}`)
		e, _ := m["error"].(map[string]any)
		if rec.Code != tc.code || e["code"] != tc.key {
			t.Errorf("%v: code=%d body=%v", tc.err, rec.Code, m)
		}
	}
	rec, _ := do(t, newServer(fakeWishlist{}, fakeHealth{}), "POST", "/api/v1/wishlist", `not json`)
	if rec.Code != 400 {
		t.Errorf("невалидный JSON: code=%d", rec.Code)
	}
}

func TestRegionsAndEmptyLists(t *testing.T) {
	h := newServer(fakeWishlist{}, fakeHealth{})
	_, m := do(t, h, "GET", "/api/v1/regions", "")
	if rs, _ := m["regions"].([]any); len(rs) != 5 {
		t.Fatalf("regions = %v", m)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/wishlist", nil))
	if strings.TrimSpace(rec.Body.String()) != `{"items":[]}` {
		t.Fatalf("пустой вишлист: %s", rec.Body.String())
	}
}

func TestUnauthorized(t *testing.T) {
	s := &Server{Wishlist: fakeWishlist{}, Health: fakeHealth{}, Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		UserID: func(*http.Request) (int64, bool) { return 0, false }}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/wishlist", nil))
	if rec.Code != 401 {
		t.Fatalf("code=%d", rec.Code)
	}
}
