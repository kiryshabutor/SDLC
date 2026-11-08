package domain

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// maxAppID ограничивает appid разумным диапазоном Steam (32-битное число).
const maxAppID = 1<<31 - 1

var storeHosts = map[string]bool{
	"store.steampowered.com": true,
	"steamcommunity.com":     true,
}

// ParseAppRef извлекает appid из ввода пользователя (REQ-FUNC-009).
// Принимаются число («1245620») и ссылки вида
// https://store.steampowered.com/app/1245620/ELDEN_RING/ (схема необязательна),
// а также страницы сообщества steamcommunity.com/app/<appid>.
func ParseAppRef(input string) (AppID, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, fmt.Errorf("%w: empty input", ErrInvalidAppRef)
	}
	if id, ok := parseID(s); ok {
		return id, nil
	}

	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidAppRef, err)
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if !storeHosts[host] {
		return 0, fmt.Errorf("%w: unsupported host %q", ErrInvalidAppRef, u.Hostname())
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "app" {
			if id, ok := parseID(parts[i+1]); ok {
				return id, nil
			}
			break
		}
	}
	return 0, fmt.Errorf("%w: no /app/<appid> in %q", ErrInvalidAppRef, input)
}

func parseID(s string) (AppID, bool) {
	if s == "" || len(s) > 10 {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > maxAppID {
		return 0, false
	}
	return AppID(n), true
}
