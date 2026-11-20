package domain

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestParseWishlistQuery(t *testing.T) {
	q, err := ParseWishlistQuery("", nil)
	if err != nil || q.Sort != "added" || q.OnlyDiscount || q.OnlyHistLow {
		t.Fatalf("%+v %v", q, err)
	}
	q, err = ParseWishlistQuery("price", []string{"discount,hist_low"})
	if err != nil || q.Sort != "price" || !q.OnlyDiscount || !q.OnlyHistLow {
		t.Fatalf("%+v %v", q, err)
	}
	q, _ = ParseWishlistQuery("name", []string{"discount", "hist_low"})
	if !q.OnlyDiscount || !q.OnlyHistLow {
		t.Fatalf("повторяющийся filter: %+v", q)
	}
	for _, tc := range [][2]string{{"rating", ""}, {"name", "cheap"}} {
		if _, err := ParseWishlistQuery(tc[0], []string{tc[1]}); !errors.Is(err, ErrInvalidSort) {
			t.Errorf("%v: %v", tc, err)
		}
	}
}

// Тестовый набор из 20 игр (критерий приёмки REQ-FUNC-013).
func testWishlist() []WishlistItem {
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var items []WishlistItem
	for i := 0; i < 20; i++ {
		it := WishlistItem{Game: Game{AppID: AppID(100 + i), Name: fmt.Sprintf("Game %02d", 19-i)}, AddedAt: t0.Add(time.Duration(i) * time.Hour)}
		switch {
		case i < 12: // продаются; у чётных скидка 5·i %
			pct := 0
			if i%2 == 0 {
				pct = 5 * i
			}
			hl := 1000 + 100*i
			it.Price = &Price{Availability: Available, Final: 1000 + 100*i, Initial: 3000, DiscountPct: pct}
			it.HistLow = &hl
			it.AtHistLow = AtHistLow(*it.Price, it.HistLow)
		case i < 15:
			it.Price = &Price{Availability: Unavailable}
		case i < 17:
			it.Price = &Price{Availability: Free}
		default: // ещё не опрашивались
		}
		items = append(items, it)
	}
	return items
}

func TestWishlistQueryApply(t *testing.T) {
	items := testWishlist()
	ids := func(list []WishlistItem) []AppID {
		out := make([]AppID, len(list))
		for i, it := range list {
			out[i] = it.AppID
		}
		return out
	}

	added := WishlistQuery{Sort: "added"}.Apply(items)
	if len(added) != 20 || added[0].AppID != 119 || added[19].AppID != 100 {
		t.Fatalf("added: %v", ids(added))
	}
	byName := WishlistQuery{Sort: "name"}.Apply(items)
	if byName[0].Name != "Game 00" || byName[19].Name != "Game 19" {
		t.Fatalf("name: %v", ids(byName))
	}
	byPrice := WishlistQuery{Sort: "price"}.Apply(items)
	if byPrice[0].AppID != 100 || byPrice[11].AppID != 111 {
		t.Fatalf("price: %v", ids(byPrice))
	}
	for _, it := range byPrice[12:] {
		if it.Price != nil && it.Price.Availability == Available {
			t.Fatalf("игры без цены должны идти в конце: %v", ids(byPrice))
		}
	}
	byDiscount := WishlistQuery{Sort: "discount"}.Apply(items)
	if byDiscount[0].AppID != 110 || byDiscount[0].Price.DiscountPct != 50 || byDiscount[5].Price.DiscountPct != 0 {
		t.Fatalf("discount: %v", ids(byDiscount))
	}

	disc := WishlistQuery{Sort: "added", OnlyDiscount: true}.Apply(items)
	if len(disc) != 5 { // i = 2, 4, 6, 8, 10
		t.Fatalf("only discount: %v", ids(disc))
	}
	hl := WishlistQuery{Sort: "added", OnlyHistLow: true}.Apply(items)
	if len(hl) != 5 {
		t.Fatalf("only hist low: %v", ids(hl))
	}
	if len(items) != 20 || items[0].AppID != 100 {
		t.Fatal("Apply не должен менять исходный список")
	}
}
