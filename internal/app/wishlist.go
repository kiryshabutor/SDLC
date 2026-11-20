// Package app — сценарии использования (прецеденты). Зависит только от
// интерфейсов source и от узкого интерфейса хранилища (DV-003).
package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/source"
)

// WishlistStore — операции хранилища, нужные сценариям вишлиста.
type WishlistStore interface {
	UserRegion(ctx context.Context, userID int64) (domain.Region, error)
	WishlistStatus(ctx context.Context, userID int64, appID domain.AppID) (exists bool, count int, err error)
	InWishlist(ctx context.Context, userID int64, appIDs []domain.AppID) (map[domain.AppID]bool, error)
	AddToWishlist(ctx context.Context, userID int64, region domain.Region, g domain.Game, p domain.Price, now time.Time) (domain.WishlistItem, error)
	Wishlist(ctx context.Context, userID int64) ([]domain.WishlistItem, error)
	RemoveFromWishlist(ctx context.Context, userID int64, appID domain.AppID) error
	SetCondition(ctx context.Context, userID int64, appID domain.AppID, c domain.ItemCondition) error
}

// Wishlist реализует UC-04 «Найти игру», UC-05 «Добавить игру в вишлист»
// (диаграммы AD-02, SD-01), UC-07 «Задать условие уведомления» и UC-08
// «Просмотреть вишлист».
type Wishlist struct {
	Store  WishlistStore
	Search source.GameSearch
	Now    func() time.Time
}

// SearchLimit — максимум результатов поиска (REQ-FUNC-008).
const SearchLimit = 20

// Find ищет игры в регионе пользователя и отмечает уже добавленные.
func (w *Wishlist) Find(ctx context.Context, userID int64, query string) ([]domain.GameHit, error) {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < domain.MinSearchQuery {
		return nil, domain.ErrQueryTooShort
	}
	region, err := w.Store.UserRegion(ctx, userID)
	if err != nil {
		return nil, err
	}
	hits, err := w.Search.Search(ctx, region, query, SearchLimit)
	if err != nil {
		return nil, err
	}
	ids := make([]domain.AppID, len(hits))
	for i, h := range hits {
		ids[i] = h.AppID
	}
	in, err := w.Store.InWishlist(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	for i := range hits {
		hits[i].InWishlist = in[hits[i].AppID]
	}
	return hits, nil
}

// Add добавляет игру по ссылке или appid. Порядок проверок повторяет AD-02:
// всё, что не требует Steam, проверяется до запроса, чтобы не тратить бюджет.
func (w *Wishlist) Add(ctx context.Context, userID int64, ref string) (domain.WishlistItem, error) {
	appID, err := domain.ParseAppRef(ref)
	if err != nil {
		return domain.WishlistItem{}, err
	}
	exists, count, err := w.Store.WishlistStatus(ctx, userID, appID)
	if err != nil {
		return domain.WishlistItem{}, err
	}
	if exists {
		return domain.WishlistItem{}, domain.ErrAlreadyInWishlist
	}
	if count >= domain.MaxWishlistSize {
		return domain.WishlistItem{}, domain.ErrWishlistFull
	}
	region, err := w.Store.UserRegion(ctx, userID)
	if err != nil {
		return domain.WishlistItem{}, err
	}
	game, price, err := w.Search.Details(ctx, region, appID)
	if err != nil {
		return domain.WishlistItem{}, err
	}
	if game.Name == "" {
		return domain.WishlistItem{}, errors.Join(domain.ErrGameNotFound, errors.New("empty name"))
	}
	game.AppID = appID
	// Задание загрузки истории ITAD для US и PL (UC-17) ставится в F-14.
	return w.Store.AddToWishlist(ctx, userID, region, game, price, w.now())
}

// List возвращает вишлист пользователя с сортировкой и фильтрами
// (REQ-FUNC-012, REQ-FUNC-013).
func (w *Wishlist) List(ctx context.Context, userID int64, q domain.WishlistQuery) ([]domain.WishlistItem, error) {
	items, err := w.Store.Wishlist(ctx, userID)
	if err != nil {
		return nil, err
	}
	return q.Apply(items), nil
}

// Remove удаляет игру из вишлиста (REQ-FUNC-014).
func (w *Wishlist) Remove(ctx context.Context, userID int64, appID domain.AppID) error {
	return w.Store.RemoveFromWishlist(ctx, userID, appID)
}

// SetCondition проверяет и сохраняет условие уведомления (REQ-FUNC-015).
func (w *Wishlist) SetCondition(ctx context.Context, userID int64, appID domain.AppID, typ string, value int) (domain.ItemCondition, error) {
	c, err := domain.ParseCondition(typ, value)
	if err != nil {
		return domain.ItemCondition{}, err
	}
	return c, w.Store.SetCondition(ctx, userID, appID, c)
}

func (w *Wishlist) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC().Truncate(time.Microsecond)
	}
	return time.Now().UTC().Truncate(time.Microsecond)
}
