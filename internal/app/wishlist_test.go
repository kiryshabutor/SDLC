package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/source/mock"
)

// memStore — хранилище в памяти для проверки сценариев без БД.
type memStore struct {
	region domain.Region
	items  map[domain.AppID]domain.WishlistItem
}

func newMem() *memStore {
	return &memStore{region: domain.RegionBY, items: map[domain.AppID]domain.WishlistItem{}}
}

func (m *memStore) UserRegion(context.Context, int64) (domain.Region, error) { return m.region, nil }
func (m *memStore) WishlistStatus(_ context.Context, _ int64, id domain.AppID) (bool, int, error) {
	_, ok := m.items[id]
	return ok, len(m.items), nil
}
func (m *memStore) InWishlist(_ context.Context, _ int64, ids []domain.AppID) (map[domain.AppID]bool, error) {
	out := map[domain.AppID]bool{}
	for _, id := range ids {
		if _, ok := m.items[id]; ok {
			out[id] = true
		}
	}
	return out, nil
}
func (m *memStore) AddToWishlist(_ context.Context, _ int64, r domain.Region, g domain.Game, p domain.Price, now time.Time) (domain.WishlistItem, error) {
	it := domain.WishlistItem{Game: g, Region: r, Condition: domain.ConditionAny, Price: p, AddedAt: now, CheckedAt: now}
	m.items[g.AppID] = it
	return it, nil
}
func (m *memStore) Wishlist(context.Context, int64) ([]domain.WishlistItem, error) {
	var out []domain.WishlistItem
	for _, it := range m.items {
		out = append(out, it)
	}
	return out, nil
}

// countingSearch считает обращения к источнику, чтобы проверить порядок AD-02.
type countingSearch struct {
	*mock.Source
	details int
}

func (c *countingSearch) Details(ctx context.Context, r domain.Region, id domain.AppID) (domain.Game, domain.Price, error) {
	c.details++
	return c.Source.Details(ctx, r, id)
}

func TestAddByLinkAndAppID(t *testing.T) {
	src := &countingSearch{Source: mock.New()}
	w := &Wishlist{Store: newMem(), Search: src}
	ctx := context.Background()

	it, err := w.Add(ctx, 1, "https://store.steampowered.com/app/1245620/ELDEN_RING/")
	if err != nil || it.AppID != 1245620 || it.Condition != domain.ConditionAny || it.Price.Final != 5999 {
		t.Fatalf("item=%+v err=%v", it, err)
	}
	if _, err := w.Add(ctx, 1, "292030"); err != nil {
		t.Fatal(err)
	}
	// Поток E2 UC-05: повтор отклоняется без обращения к Steam.
	before := src.details
	if _, err := w.Add(ctx, 1, "1245620"); !errors.Is(err, domain.ErrAlreadyInWishlist) {
		t.Fatalf("err = %v", err)
	}
	if src.details != before {
		t.Error("повторное добавление не должно обращаться к Steam")
	}
}

func TestAddErrors(t *testing.T) {
	src := &countingSearch{Source: mock.New()}
	store := newMem()
	w := &Wishlist{Store: store, Search: src}
	ctx := context.Background()

	if _, err := w.Add(ctx, 1, "elden ring"); !errors.Is(err, domain.ErrInvalidAppRef) {
		t.Errorf("E1: err = %v", err)
	}
	if _, err := w.Add(ctx, 1, "777"); !errors.Is(err, domain.ErrGameNotFound) {
		t.Errorf("E4: err = %v", err)
	}
	for i := 0; i < domain.MaxWishlistSize; i++ {
		store.items[domain.AppID(100000+i)] = domain.WishlistItem{}
	}
	before := src.details
	if _, err := w.Add(ctx, 1, "1245620"); !errors.Is(err, domain.ErrWishlistFull) {
		t.Errorf("E3: err = %v", err)
	}
	if src.details != before {
		t.Error("при полном вишлисте Steam не запрашивается")
	}
}

func TestFind(t *testing.T) {
	store := newMem()
	w := &Wishlist{Store: store, Search: mock.New()}
	ctx := context.Background()
	if _, err := w.Find(ctx, 1, " e "); !errors.Is(err, domain.ErrQueryTooShort) {
		t.Fatalf("err = %v", err)
	}
	if _, err := w.Add(ctx, 1, "1245620"); err != nil {
		t.Fatal(err)
	}
	hits, err := w.Find(ctx, 1, "elden")
	if err != nil || len(hits) != 1 || !hits[0].InWishlist {
		t.Fatalf("hits=%+v err=%v", hits, err)
	}
}
