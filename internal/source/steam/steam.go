// Package steam — адаптер Steam Store API к интерфейсам source.
// Здесь ответы Steam переводятся в доменные типы, а особые состояния игры
// (бесплатная, не вышла, недоступна) определяются до бизнес-логики (DV-006).
package steam

import (
	"context"
	"errors"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/platform"
	"github.com/kiryshabutor/SDLC/pkg/steamstore"
)

// Source реализует source.GameSearch и source.PriceSource.
type Source struct {
	client  *steamstore.Client
	limiter *platform.Window
	backoff time.Duration
}

// New создаёт адаптер. limiter — бюджет запросов процесса (API: 30 за 5 мин).
func New(client *steamstore.Client, limiter *platform.Window) *Source {
	return &Source{client: client, limiter: limiter, backoff: time.Second}
}

// do выполняет запрос с учётом бюджета и повторов 1, 2, 4 с (REQ-REL-001).
func (s *Source) do(ctx context.Context, fn func() error) error {
	return platform.Retry(ctx, 3, s.backoff, func() error {
		if s.limiter != nil && !s.limiter.Allow() {
			return domain.ErrSourceUnavailable
		}
		err := fn()
		var se *steamstore.StatusError
		if errors.As(err, &se) && (se.Code == 429 || se.Code == 403) && s.limiter != nil {
			pause := se.RetryAfter
			if pause < time.Minute {
				pause = time.Minute
			}
			s.limiter.Pause(pause)
		}
		return err
	})
}

// Search ищет игры; результаты, не являющиеся приложениями, отбрасываются.
func (s *Source) Search(ctx context.Context, region domain.Region, query string, limit int) ([]domain.GameHit, error) {
	var items []steamstore.SearchItem
	err := s.do(ctx, func() (err error) {
		items, err = s.client.Search(ctx, string(region), query)
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	hits := make([]domain.GameHit, 0, len(items))
	for _, it := range items {
		if it.Type != "app" {
			continue
		}
		h := domain.GameHit{Game: domain.Game{AppID: domain.AppID(it.ID), Name: it.Name, CoverURL: it.TinyImage}}
		if it.Price != nil {
			h.Price = &domain.Price{
				Availability: domain.Available, Final: it.Price.Final, Initial: it.Price.Initial,
				DiscountPct: discount(it.Price.Initial, it.Price.Final), Currency: it.Price.Currency,
			}
		}
		hits = append(hits, h)
		if len(hits) == limit {
			break
		}
	}
	return hits, nil
}

// Details возвращает игру и её цену в регионе.
func (s *Source) Details(ctx context.Context, region domain.Region, appID domain.AppID) (domain.Game, domain.Price, error) {
	var app steamstore.App
	err := s.do(ctx, func() (err error) {
		app, err = s.client.App(ctx, string(region), int(appID))
		return err
	})
	if errors.Is(err, steamstore.ErrNotFound) {
		// Steam отвечает success=false и для несуществующей игры, и для игры,
		// недоступной в регионе. Различить их без второго запроса нельзя;
		// для добавления в вишлист оба случая — «Игра не найдена в Steam».
		return domain.Game{}, domain.Price{}, domain.ErrGameNotFound
	}
	if err != nil {
		return domain.Game{}, domain.Price{}, wrap(err)
	}
	return domain.Game{AppID: appID, Name: app.Name, CoverURL: app.HeaderImage}, toPrice(app), nil
}

// Prices получает цены пакета игр одного региона. Игры, которых нет в ответе,
// считаются недоступными в регионе (REQ-FUNC-016).
func (s *Source) Prices(ctx context.Context, region domain.Region, appIDs []domain.AppID) (map[domain.AppID]domain.Price, error) {
	ids := make([]int, len(appIDs))
	for i, id := range appIDs {
		ids[i] = int(id)
	}
	var apps map[int]steamstore.App
	err := s.do(ctx, func() (err error) {
		apps, err = s.client.AppDetails(ctx, string(region), ids...)
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	out := make(map[domain.AppID]domain.Price, len(appIDs))
	for _, id := range appIDs {
		app, ok := apps[int(id)]
		if !ok {
			out[id] = domain.Price{Availability: domain.Unavailable}
			continue
		}
		out[id] = toPrice(app)
	}
	return out, nil
}

func toPrice(app steamstore.App) domain.Price {
	switch {
	case app.Price != nil:
		return domain.Price{
			Availability: domain.Available, Final: app.Price.Final, Initial: app.Price.Initial,
			DiscountPct: app.Price.DiscountPercent, Currency: app.Price.Currency,
		}
	case app.IsFree:
		return domain.Price{Availability: domain.Free}
	case app.ReleaseDate.ComingSoon:
		return domain.Price{Availability: domain.Unreleased}
	default:
		// Пакетный запрос с filters=price_overview не возвращает is_free:
		// игра без цены считается бесплатной.
		return domain.Price{Availability: domain.Free}
	}
}

func discount(initial, final int) int {
	if initial <= 0 || final >= initial {
		return 0
	}
	return (initial - final) * 100 / initial
}

func wrap(err error) error {
	if errors.Is(err, domain.ErrSourceUnavailable) || errors.Is(err, context.Canceled) {
		return err
	}
	return errors.Join(domain.ErrSourceUnavailable, err)
}
