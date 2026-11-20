package domain

// Условия уведомления (REQ-FUNC-015, DV-010). Чистые функции без ввода-вывода.

// Ограничения значений условий.
const (
	MinDiscountPct = 1
	MaxDiscountPct = 99
	// MaxBelowPrice — верхняя граница X в минимальных единицах валюты
	// (10 000 000 = 100 000,00): защита от опечаток и переполнения.
	MaxBelowPrice = 10_000_000
)

// ItemCondition — условие уведомления элемента вишлиста: тип и значение.
// Value — N для min_pct (1–99) и X для below в минимальных единицах валюты
// региона; для any и hist_low значение равно 0.
type ItemCondition struct {
	Type  Condition `json:"type"`
	Value int       `json:"value,omitempty"`
}

// ParseCondition проверяет тип и значение условия.
func ParseCondition(typ string, value int) (ItemCondition, error) {
	c := ItemCondition{Type: Condition(typ), Value: value}
	switch c.Type {
	case ConditionAny, ConditionHistLow:
		c.Value = 0
	case ConditionMinPct:
		if value < MinDiscountPct || value > MaxDiscountPct {
			return ItemCondition{}, ErrInvalidConditionValue
		}
	case ConditionBelow:
		if value <= 0 || value > MaxBelowPrice {
			return ItemCondition{}, ErrInvalidConditionValue
		}
	default:
		return ItemCondition{}, ErrInvalidCondition
	}
	return c, nil
}

// Met сообщает, выполнено ли условие для цены по таблице решений DV-010.
// histLow — исторический минимум до учёта текущей цены, nil — минимум не
// определён. Для игр в особых состояниях условие не проверяется
// (REQ-FUNC-016).
func (c ItemCondition) Met(p Price, histLow *int) bool {
	if p.Availability != Available {
		return false
	}
	switch c.Type {
	case ConditionAny:
		return p.DiscountPct > 0
	case ConditionMinPct:
		return p.DiscountPct >= c.Value
	case ConditionBelow:
		return p.Final <= c.Value
	case ConditionHistLow:
		return p.DiscountPct > 0 && (histLow == nil || p.Final <= *histLow)
	}
	return false
}

// AtHistLow — отметка «исторический минимум сейчас» в вишлисте
// (REQ-FUNC-012): игра продаётся со скидкой не дороже известного минимума.
func AtHistLow(p Price, histLow *int) bool {
	return p.Availability == Available && p.DiscountPct > 0 && histLow != nil && p.Final <= *histLow
}

// Evaluate применяет защиту от повторов (REQ-FUNC-026, DV-010, SM-01b):
// возвращает, нужно ли уведомить, и новое значение last_notified_price.
// Если условие перестало выполняться, last_notified_price сбрасывается,
// чтобы следующая скидка снова дала уведомление.
func Evaluate(c ItemCondition, p Price, histLow, lastNotified *int) (notify bool, newLast *int) {
	if p.Availability != Available {
		return false, lastNotified
	}
	if !c.Met(p, histLow) {
		return false, nil
	}
	if lastNotified != nil && *lastNotified == p.Final {
		return false, lastNotified
	}
	final := p.Final
	return true, &final
}
