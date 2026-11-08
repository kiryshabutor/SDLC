// Package source задаёт интерфейсы источников данных (DV-006, REQ-MAINT-001).
// Бизнес-логика зависит только от них; реализации — в подпакетах steam, mock.
package source

import (
	"context"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// GameSearch — поиск игр и сведения об одной игре в регионе.
type GameSearch interface {
	Search(ctx context.Context, region domain.Region, query string, limit int) ([]domain.GameHit, error)
	// Details возвращает domain.ErrGameNotFound, если Steam не знает appid.
	Details(ctx context.Context, region domain.Region, appID domain.AppID) (domain.Game, domain.Price, error)
}

// PriceSource — пакетное получение цен для цикла опроса (F-08, итерация 4).
type PriceSource interface {
	Prices(ctx context.Context, region domain.Region, appIDs []domain.AppID) (map[domain.AppID]domain.Price, error)
}
