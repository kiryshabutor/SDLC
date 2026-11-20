package domain

import (
	"errors"
	"testing"
)

func ptr(v int) *int { return &v }

func TestParseCondition(t *testing.T) {
	ok := []struct {
		typ   string
		value int
		want  ItemCondition
	}{
		{"any", 0, ItemCondition{ConditionAny, 0}},
		{"any", 42, ItemCondition{ConditionAny, 0}},
		{"hist_low", 0, ItemCondition{ConditionHistLow, 0}},
		{"min_pct", 1, ItemCondition{ConditionMinPct, 1}},
		{"min_pct", 99, ItemCondition{ConditionMinPct, 99}},
		{"below", 1, ItemCondition{ConditionBelow, 1}},
		{"below", 999, ItemCondition{ConditionBelow, 999}},
	}
	for _, tc := range ok {
		got, err := ParseCondition(tc.typ, tc.value)
		if err != nil || got != tc.want {
			t.Errorf("%s(%d): %+v %v", tc.typ, tc.value, got, err)
		}
	}
	bad := []struct {
		typ   string
		value int
		want  error
	}{
		{"min_pct", 0, ErrInvalidConditionValue},
		{"min_pct", 100, ErrInvalidConditionValue},
		{"min_pct", -5, ErrInvalidConditionValue},
		{"below", 0, ErrInvalidConditionValue},
		{"below", -100, ErrInvalidConditionValue},
		{"below", MaxBelowPrice + 1, ErrInvalidConditionValue},
		{"daily", 0, ErrInvalidCondition},
		{"", 0, ErrInvalidCondition},
	}
	for _, tc := range bad {
		if _, err := ParseCondition(tc.typ, tc.value); !errors.Is(err, tc.want) {
			t.Errorf("%s(%d): got %v, want %v", tc.typ, tc.value, err, tc.want)
		}
	}
}

// Граничные значения из критерия приёмки REQ-FUNC-015.
func TestConditionMet(t *testing.T) {
	price := func(final, pct int) Price {
		return Price{Availability: Available, Final: final, Initial: 10000, DiscountPct: pct, Currency: "USD"}
	}
	cases := []struct {
		name    string
		c       ItemCondition
		p       Price
		histLow *int
		want    bool
	}{
		{"any: 0 %", ItemCondition{ConditionAny, 0}, price(10000, 0), nil, false},
		{"any: 1 %", ItemCondition{ConditionAny, 0}, price(9900, 1), nil, true},
		{"min_pct 30: 29 %", ItemCondition{ConditionMinPct, 30}, price(7100, 29), nil, false},
		{"min_pct 30: 30 %", ItemCondition{ConditionMinPct, 30}, price(7000, 30), nil, true},
		{"min_pct 30: 31 %", ItemCondition{ConditionMinPct, 30}, price(6900, 31), nil, true},
		{"below 1000: 1001", ItemCondition{ConditionBelow, 1000}, price(1001, 90), nil, false},
		{"below 1000: 1000", ItemCondition{ConditionBelow, 1000}, price(1000, 90), nil, true},
		{"below 1000: без скидки, но дешевле", ItemCondition{ConditionBelow, 1000}, Price{Availability: Available, Final: 999, Initial: 999}, nil, true},
		{"hist_low: минимум не определён", ItemCondition{ConditionHistLow, 0}, price(5000, 50), nil, true},
		{"hist_low: равен минимуму", ItemCondition{ConditionHistLow, 0}, price(5000, 50), ptr(5000), true},
		{"hist_low: дороже минимума", ItemCondition{ConditionHistLow, 0}, price(5001, 50), ptr(5000), false},
		{"hist_low: без скидки", ItemCondition{ConditionHistLow, 0}, price(4000, 0), ptr(5000), false},
		{"недоступна", ItemCondition{ConditionAny, 0}, Price{Availability: Unavailable, DiscountPct: 50}, nil, false},
		{"бесплатная", ItemCondition{ConditionBelow, 1000}, Price{Availability: Free}, nil, false},
		{"не вышла", ItemCondition{ConditionHistLow, 0}, Price{Availability: Unreleased, DiscountPct: 10}, nil, false},
		{"неизвестный тип", ItemCondition{Condition("x"), 0}, price(1, 99), nil, false},
	}
	for _, tc := range cases {
		if got := tc.c.Met(tc.p, tc.histLow); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestAtHistLow(t *testing.T) {
	p := Price{Availability: Available, Final: 499, Initial: 1999, DiscountPct: 75}
	if !AtHistLow(p, ptr(499)) || AtHistLow(p, ptr(400)) || AtHistLow(p, nil) {
		t.Fatal("AtHistLow")
	}
	p.DiscountPct = 0
	if AtHistLow(p, ptr(999)) {
		t.Fatal("без скидки отметки нет")
	}
}

// Защита от повторов (REQ-FUNC-026): по одной цене не более одного уведомления.
func TestEvaluate(t *testing.T) {
	c := ItemCondition{ConditionAny, 0}
	sale := Price{Availability: Available, Final: 1249, Initial: 2499, DiscountPct: 50}
	notify, last := Evaluate(c, sale, nil, nil)
	if !notify || last == nil || *last != 1249 {
		t.Fatalf("первая скидка: %v %v", notify, last)
	}
	if notify, last2 := Evaluate(c, sale, nil, last); notify || last2 != last {
		t.Fatal("та же цена — без уведомления")
	}
	deeper := sale
	deeper.Final, deeper.DiscountPct = 999, 60
	if notify, l := Evaluate(c, deeper, nil, last); !notify || *l != 999 {
		t.Fatal("цена изменилась и условие выполнено — новое уведомление")
	}
	full := Price{Availability: Available, Final: 2499, Initial: 2499}
	if notify, l := Evaluate(c, full, nil, last); notify || l != nil {
		t.Fatal("скидка закончилась — сброс last_notified_price")
	}
	gone := Price{Availability: Unavailable}
	if notify, l := Evaluate(c, gone, nil, last); notify || l != last {
		t.Fatal("в особом состоянии условие не проверяется и состояние не меняется")
	}
}
