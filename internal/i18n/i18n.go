// Package i18n — файлы локализации текстов, которые видит пользователь
// (REQ-INT-001, F-21). Тексты лежат в locales/<язык>.json и встраиваются в
// бинарный файл; код ссылается на них по ключу. В R1 есть только русский
// язык; английский в R2 добавляется файлом en.json без изменения кода.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed locales/*.json
var files embed.FS

// DefaultLang — язык R1.
const DefaultLang = "ru"

// Catalog — тексты одного языка.
type Catalog struct {
	lang     string
	messages map[string]string
}

// Load читает каталог языка из встроенных файлов.
func Load(lang string) (*Catalog, error) {
	data, err := files.ReadFile("locales/" + lang + ".json")
	if err != nil {
		return nil, fmt.Errorf("i18n: no locale %q: %w", lang, err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("i18n: %s.json: %w", lang, err)
	}
	for k, v := range m {
		if strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("i18n: %s.json: empty text for %q", lang, k)
		}
	}
	return &Catalog{lang: lang, messages: m}, nil
}

// MustLoad — Load для инициализации при старте; ошибка означает дефект сборки.
func MustLoad(lang string) *Catalog {
	c, err := Load(lang)
	if err != nil {
		panic(err)
	}
	return c
}

// T возвращает текст по ключу. Отсутствующий ключ возвращается как есть,
// чтобы пропуск был заметен в интерфейсе; тест TestKeysPresent не даёт
// такому коду попасть в main.
func (c *Catalog) T(key string) string {
	if v, ok := c.messages[key]; ok {
		return v
	}
	return key
}

// Has сообщает, есть ли ключ в каталоге.
func (c *Catalog) Has(key string) bool {
	_, ok := c.messages[key]
	return ok
}

// Keys возвращает отсортированный список ключей (для тестов полноты).
func (c *Catalog) Keys() []string {
	keys := make([]string, 0, len(c.messages))
	for k := range c.messages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Lang возвращает код языка каталога.
func (c *Catalog) Lang() string { return c.lang }
