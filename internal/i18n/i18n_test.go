package i18n

import (
	"regexp"
	"testing"
)

func TestLoadRussian(t *testing.T) {
	c, err := Load(DefaultLang)
	if err != nil {
		t.Fatal(err)
	}
	if c.Lang() != "ru" || c.T("error.invalid_credentials") != "Неверный email или пароль" {
		t.Fatalf("T = %q", c.T("error.invalid_credentials"))
	}
	if c.T("no.such.key") != "no.such.key" || c.Has("no.such.key") {
		t.Fatal("отсутствующий ключ должен возвращаться как есть")
	}
	if _, err := Load("xx"); err == nil {
		t.Fatal("ожидалась ошибка для неизвестного языка")
	}
}

// REQ-INT-001: тексты на русском. Каждый текст содержит кириллицу; латиница
// допустима только внутри (email, Steam, appid, X).
func TestTextsAreRussian(t *testing.T) {
	c := MustLoad(DefaultLang)
	cyr := regexp.MustCompile(`\p{Cyrillic}`)
	for _, k := range c.Keys() {
		if !cyr.MatchString(c.T(k)) {
			t.Errorf("%s: текст без кириллицы: %q", k, c.T(k))
		}
	}
}
