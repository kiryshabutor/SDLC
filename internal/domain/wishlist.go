package domain

import (
	"sort"
	"strings"
)

// Сортировка и фильтры вишлиста (REQ-FUNC-013). Вишлист ограничен 500
// играми, поэтому сортировка выполняется в памяти над результатом запроса.

// WishlistQuery — параметры GET /wishlist?sort=&filter=.
type WishlistQuery struct {
	Sort         string // added (по умолчанию), name, price, discount
	OnlyDiscount bool   // filter=discount
	OnlyHistLow  bool   // filter=hist_low
}

// ParseWishlistQuery проверяет параметры. filter может повторяться или
// содержать значения через запятую.
func ParseWishlistQuery(sortBy string, filters []string) (WishlistQuery, error) {
	q := WishlistQuery{Sort: sortBy}
	switch q.Sort {
	case "":
		q.Sort = "added"
	case "added", "name", "price", "discount":
	default:
		return WishlistQuery{}, ErrInvalidSort
	}
	for _, f := range filters {
		for _, v := range strings.Split(f, ",") {
			switch strings.TrimSpace(v) {
			case "":
			case "discount":
				q.OnlyDiscount = true
			case "hist_low":
				q.OnlyHistLow = true
			default:
				return WishlistQuery{}, ErrInvalidSort
			}
		}
	}
	return q, nil
}

// Apply фильтрует и сортирует копию списка. Игры без цены (не опрашивались,
// недоступны, бесплатны, не вышли) при сортировке по цене и скидке идут в
// конце. При равенстве порядок — по названию, затем по appid.
func (q WishlistQuery) Apply(items []WishlistItem) []WishlistItem {
	out := make([]WishlistItem, 0, len(items))
	for _, it := range items {
		if q.OnlyDiscount && (it.Price == nil || it.Price.Availability != Available || it.Price.DiscountPct == 0) {
			continue
		}
		if q.OnlyHistLow && !it.AtHistLow {
			continue
		}
		out = append(out, it)
	}
	priced := func(it WishlistItem) bool { return it.Price != nil && it.Price.Availability == Available }
	byName := func(a, b WishlistItem) bool {
		an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if an != bn {
			return an < bn
		}
		return a.AppID < b.AppID
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch q.Sort {
		case "name":
			return byName(a, b)
		case "price", "discount":
			if priced(a) != priced(b) {
				return priced(a)
			}
			if priced(a) {
				if q.Sort == "price" && a.Price.Final != b.Price.Final {
					return a.Price.Final < b.Price.Final
				}
				if q.Sort == "discount" && a.Price.DiscountPct != b.Price.DiscountPct {
					return a.Price.DiscountPct > b.Price.DiscountPct
				}
			}
			return byName(a, b)
		default: // added: новые сверху
			if !a.AddedAt.Equal(b.AddedAt) {
				return a.AddedAt.After(b.AddedAt)
			}
			return a.AppID < b.AppID
		}
	})
	return out
}
