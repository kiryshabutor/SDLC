package domain

import "time"

// AppID — числовой идентификатор игры в Steam.
type AppID int

// Availability — особое состояние игры в регионе (REQ-FUNC-016).
type Availability string

// Значения Availability совпадают со значениями столбца price_state.availability.
const (
	Available   Availability = "available"
	Unavailable Availability = "unavailable"
	Free        Availability = "free"
	Unreleased  Availability = "unreleased"
)

// Game — сведения об игре, общие для всех регионов.
type Game struct {
	AppID    AppID  `json:"appid"`
	Name     string `json:"name"`
	CoverURL string `json:"cover_url"`
}

// Price — цена игры в регионе в минимальных единицах валюты (3999 = 39,99).
type Price struct {
	Availability Availability `json:"availability"`
	Final        int          `json:"final"`
	Initial      int          `json:"initial"`
	DiscountPct  int          `json:"discount_pct"`
	Currency     string       `json:"currency"`
}

// Changed сообщает, нужно ли записывать новую точку истории (ADR-0004):
// изменилась итоговая цена, скидка или доступность.
func (p Price) Changed(prev Price) bool {
	return p.Final != prev.Final || p.DiscountPct != prev.DiscountPct ||
		p.Availability != prev.Availability || p.Initial != prev.Initial
}

// GameHit — строка результата поиска (REQ-FUNC-008).
type GameHit struct {
	Game
	Price      *Price `json:"price,omitempty"`
	InWishlist bool   `json:"in_wishlist"`
}

// Condition — условие уведомления (REQ-FUNC-015). В итерации 1 используется
// только значение по умолчанию; проверка условий появится в F-07 и F-09.
type Condition string

// Типы условий (столбец wishlist_items.cond_type).
const (
	ConditionAny     Condition = "any"      // любая скидка, по умолчанию
	ConditionMinPct  Condition = "min_pct"  // скидка не менее N %
	ConditionBelow   Condition = "below"    // цена не выше X; сбрасывается при смене региона
	ConditionHistLow Condition = "hist_low" // исторический минимум
)

// WishlistItem — элемент вишлиста вместе с текущей ценой пары. Price и
// CheckedAt равны nil, пока пара (игра, регион) ещё не опрашивалась: так
// бывает сразу после смены региона (REQ-FUNC-005) до ближайшего цикла.
type WishlistItem struct {
	Game
	Region    Region     `json:"region"`
	Condition Condition  `json:"condition"`
	Price     *Price     `json:"price"`
	CheckedAt *time.Time `json:"checked_at"`
	AddedAt   time.Time  `json:"added_at"`
}

// MaxWishlistSize — лимит вишлиста одного пользователя (REQ-FUNC-011).
const MaxWishlistSize = 500

// MinSearchQuery — минимальная длина поискового запроса в символах.
const MinSearchQuery = 2
