package domain

import (
	"errors"
	"testing"
)

func TestParseAppRef(t *testing.T) {
	ok := []struct {
		in   string
		want AppID
	}{
		{"1245620", 1245620},
		{"  1245620\n", 1245620},
		{"https://store.steampowered.com/app/1245620/ELDEN_RING/", 1245620},
		{"https://store.steampowered.com/app/1245620", 1245620},
		{"store.steampowered.com/app/1091500/Cyberpunk_2077/?l=russian", 1091500},
		{"http://www.store.steampowered.com/app/570/", 570},
		{"https://steamcommunity.com/app/730/discussions/", 730},
		{"https://store.steampowered.com/agecheck/app/1245620/", 1245620},
	}
	for _, tc := range ok {
		got, err := ParseAppRef(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseAppRef(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}

	bad := []string{
		"", "   ", "0", "-5", "12ab", "99999999999",
		"elden ring",
		"https://example.com/app/1245620/",
		"https://store.steampowered.com/",
		"https://store.steampowered.com/app/",
		"https://store.steampowered.com/app/abc/",
		"https://store.steampowered.com/sub/12345/",
	}
	for _, in := range bad {
		if _, err := ParseAppRef(in); !errors.Is(err, ErrInvalidAppRef) {
			t.Errorf("ParseAppRef(%q) error = %v; want ErrInvalidAppRef", in, err)
		}
	}
}

func TestParseRegion(t *testing.T) {
	for _, r := range Regions {
		if got, err := ParseRegion(string(r.Code)); err != nil || got != r.Code {
			t.Errorf("ParseRegion(%q) = %q, %v", r.Code, got, err)
		}
	}
	if _, err := ParseRegion("DE"); !errors.Is(err, ErrUnknownRegion) {
		t.Errorf("ParseRegion(DE) error = %v; want ErrUnknownRegion", err)
	}
	if !RegionUS.ITADSupported() || RegionBY.ITADSupported() {
		t.Error("ITAD поддерживается только для US и PL")
	}
}

func TestPriceChanged(t *testing.T) {
	base := Price{Availability: Available, Final: 3999, Initial: 3999, Currency: "USD"}
	if base.Changed(base) {
		t.Error("та же цена не должна считаться изменением")
	}
	sale := base
	sale.Final, sale.DiscountPct = 2399, 40
	if !sale.Changed(base) {
		t.Error("скидка должна считаться изменением")
	}
	gone := base
	gone.Availability = Unavailable
	if !gone.Changed(base) {
		t.Error("смена доступности должна считаться изменением")
	}
}
