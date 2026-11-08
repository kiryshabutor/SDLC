package steamstore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	return c
}

func TestAppDetailsSingle(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/appdetails" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("appids") != "1245620" || q.Get("cc") != "BY" || q.Get("filters") != "" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"1245620":{"success":true,"data":{"type":"game","name":"ELDEN RING",
			"steam_appid":1245620,"is_free":false,"header_image":"https://cdn/h.jpg",
			"price_overview":{"currency":"USD","initial":5999,"final":3599,"discount_percent":40},
			"release_date":{"coming_soon":false,"date":"24 Feb, 2022"}}}}`))
	})
	app, err := c.App(context.Background(), "BY", 1245620)
	if err != nil {
		t.Fatal(err)
	}
	if app.Name != "ELDEN RING" || app.Price == nil || app.Price.Final != 3599 || app.Price.DiscountPercent != 40 {
		t.Fatalf("app = %+v", app)
	}
}

func TestAppDetailsNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"1091500":{"success":false}}`))
	})
	if _, err := c.App(context.Background(), "BY", 1091500); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v; want ErrNotFound", err)
	}
}

func TestAppDetailsBatchUsesPriceFilter(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filters") != "price_overview" || r.URL.Query().Get("appids") != "10,20" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		// Бесплатная игра: data — пустой массив.
		_, _ = w.Write([]byte(`{"10":{"success":true,"data":{"price_overview":{"currency":"USD","initial":999,"final":999,"discount_percent":0}}},
			"20":{"success":true,"data":[]}}`))
	})
	apps, err := c.AppDetails(context.Background(), "US", 10, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 || apps[10].Price.Final != 999 || apps[20].Price != nil {
		t.Fatalf("apps = %+v", apps)
	}
}

func TestSearch(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("term") != "elden ring" {
			t.Errorf("term = %q", r.URL.Query().Get("term"))
		}
		_, _ = w.Write([]byte(`{"total":2,"items":[
			{"type":"app","name":"ELDEN RING","id":1245620,"tiny_image":"t.jpg","price":{"currency":"USD","initial":5999,"final":5999}},
			{"type":"app","name":"ELDEN RING Demo","id":42,"tiny_image":"d.jpg"}]}`))
	})
	items, err := c.Search(context.Background(), "BY", "elden ring")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != 1245620 || items[0].Price.Final != 5999 || items[1].Price != nil {
		t.Fatalf("items = %+v", items)
	}
}

func TestStatusError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := c.Search(context.Background(), "BY", "x1")
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 429 || !se.Temporary() || se.RetryAfter != 30*time.Second {
		t.Fatalf("err = %#v", err)
	}
}
