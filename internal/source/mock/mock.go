// Package mock — тестовый источник цен для TEST_MODE (REQ-FUNC-030) и тестов.
// Цены задаются вручную; в итерации 5 (F-16) к нему добавится API /test/prices.
package mock

import (
	"context"
	"strings"
	"sync"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

type key struct {
	region domain.Region
	app    domain.AppID
}

// Source хранит игры и цены в памяти. Потокобезопасен.
type Source struct {
	mu     sync.RWMutex
	games  map[domain.AppID]domain.Game
	prices map[key]domain.Price
}

// New создаёт источник с демонстрационным набором игр.
func New() *Source {
	s := &Source{games: map[domain.AppID]domain.Game{}, prices: map[key]domain.Price{}}
	for _, g := range []struct {
		id    domain.AppID
		name  string
		price int
	}{{1245620, "ELDEN RING", 5999}, {1091500, "Cyberpunk 2077", 5999}, {292030, "The Witcher 3: Wild Hunt", 3999}} {
		s.games[g.id] = domain.Game{AppID: g.id, Name: g.name}
		for _, r := range domain.Regions {
			s.prices[key{r.Code, g.id}] = domain.Price{Availability: domain.Available, Final: g.price, Initial: g.price, Currency: r.Currency}
		}
	}
	return s
}

// SetPrice задаёт цену пары (игра, регион).
func (s *Source) SetPrice(region domain.Region, app domain.AppID, p domain.Price) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prices[key{region, app}] = p
}

// Search ищет по подстроке названия без учёта регистра.
func (s *Source) Search(_ context.Context, region domain.Region, query string, limit int) ([]domain.GameHit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.GameHit
	q := strings.ToLower(query)
	for id, g := range s.games {
		if strings.Contains(strings.ToLower(g.Name), q) {
			p := s.prices[key{region, id}]
			out = append(out, domain.GameHit{Game: g, Price: &p})
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

// Details возвращает игру или domain.ErrGameNotFound.
func (s *Source) Details(_ context.Context, region domain.Region, app domain.AppID) (domain.Game, domain.Price, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.games[app]
	if !ok {
		return domain.Game{}, domain.Price{}, domain.ErrGameNotFound
	}
	return g, s.prices[key{region, app}], nil
}

// Prices возвращает заданные цены пакета.
func (s *Source) Prices(_ context.Context, region domain.Region, apps []domain.AppID) (map[domain.AppID]domain.Price, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[domain.AppID]domain.Price, len(apps))
	for _, a := range apps {
		p, ok := s.prices[key{region, a}]
		if !ok {
			p = domain.Price{Availability: domain.Unavailable}
		}
		out[a] = p
	}
	return out, nil
}
