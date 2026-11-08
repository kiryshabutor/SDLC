package steam

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/platform"
	"github.com/kiryshabutor/SDLC/pkg/steamstore"
)

func newSource(t *testing.T, h http.HandlerFunc, limit int) *Source {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := steamstore.New()
	c.BaseURL = srv.URL
	s := New(c, platform.NewWindow(limit, 5*time.Minute))
	s.backoff = time.Millisecond
	return s
}

func TestDetailsStates(t *testing.T) {
	bodies := map[string]string{
		"1245620": `{"1245620":{"success":true,"data":{"name":"ELDEN RING","header_image":"h","price_overview":{"currency":"USD","initial":5999,"final":3599,"discount_percent":40}}}}`,
		"570":     `{"570":{"success":true,"data":{"name":"Dota 2","is_free":true}}}`,
		"999":     `{"999":{"success":true,"data":{"name":"Soon","release_date":{"coming_soon":true}}}}`,
		"1091500": `{"1091500":{"success":false}}`,
	}
	s := newSource(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(bodies[r.URL.Query().Get("appids")]))
	}, 100)
	ctx := context.Background()

	g, p, err := s.Details(ctx, domain.RegionBY, 1245620)
	if err != nil || g.Name != "ELDEN RING" || p.Availability != domain.Available || p.Final != 3599 || p.DiscountPct != 40 {
		t.Fatalf("elden: %+v %+v %v", g, p, err)
	}
	if _, p, _ := s.Details(ctx, domain.RegionBY, 570); p.Availability != domain.Free {
		t.Errorf("dota: %+v", p)
	}
	if _, p, _ := s.Details(ctx, domain.RegionBY, 999); p.Availability != domain.Unreleased {
		t.Errorf("soon: %+v", p)
	}
	if _, _, err := s.Details(ctx, domain.RegionBY, 1091500); !errors.Is(err, domain.ErrGameNotFound) {
		t.Errorf("cyberpunk BY: err = %v", err)
	}
}

func TestRetryOn5xxAndBudget(t *testing.T) {
	var calls atomic.Int32
	s := newSource(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"type":"app","id":1,"name":"A","price":{"currency":"USD","initial":1000,"final":750}},{"type":"sub","id":2,"name":"Bundle"}]}`))
	}, 3)
	hits, err := s.Search(context.Background(), domain.RegionUS, "aa", 20)
	if err != nil || len(hits) != 1 || hits[0].Price.DiscountPct != 25 {
		t.Fatalf("hits=%+v err=%v calls=%d", hits, err, calls.Load())
	}
	// Бюджет 3 запроса исчерпан тремя попытками выше.
	if _, err := s.Search(context.Background(), domain.RegionUS, "aa", 20); !errors.Is(err, domain.ErrSourceUnavailable) {
		t.Fatalf("ожидался ErrSourceUnavailable, получено %v", err)
	}
}

func TestPricesMarksMissingAsUnavailable(t *testing.T) {
	s := newSource(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"1":{"success":true,"data":{"price_overview":{"currency":"USD","initial":100,"final":100,"discount_percent":0}}},"2":{"success":false}}`))
	}, 10)
	got, err := s.Prices(context.Background(), domain.RegionBY, []domain.AppID{1, 2})
	if err != nil || got[1].Final != 100 || got[2].Availability != domain.Unavailable {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
