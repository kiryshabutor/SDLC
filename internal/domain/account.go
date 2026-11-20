package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Ограничения учётной записи (REQ-FUNC-001, REQ-SEC-003, REQ-SEC-005).
const (
	MinPasswordLen   = 8
	MaxPasswordLen   = 128 // защита от хэширования мегабайтных строк
	MaxEmailLen      = 254 // RFC 5321
	SessionIdleTTL   = 30 * 24 * time.Hour
	MaxLoginFailures = 5
	LoginWindow      = 15 * time.Minute
	LoginLockout     = 15 * time.Minute
)

// NormalizeEmail проверяет формат адреса и приводит его к нижнему регистру.
// Отображаемое имя («Иван <ivan@mail.com>») не допускается.
func NormalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || len(s) > MaxEmailLen {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || addr.Name != "" {
		return "", ErrInvalidEmail
	}
	at := strings.LastIndexByte(s, '@')
	if !strings.Contains(s[at+1:], ".") {
		return "", ErrInvalidEmail
	}
	return s, nil
}

// ValidatePassword проверяет правило REQ-FUNC-001: не менее 8 символов,
// хотя бы одна буква и одна цифра.
func ValidatePassword(p string) error {
	n := utf8.RuneCountInString(p)
	if n < MinPasswordLen || n > MaxPasswordLen {
		return ErrWeakPassword
	}
	var letter, digit bool
	for _, r := range p {
		switch {
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !letter || !digit {
		return ErrWeakPassword
	}
	return nil
}

// Registration — данные формы регистрации.
type Registration struct {
	Email           string
	Password        string
	PasswordConfirm string
	Region          string
	Consent         bool
}

// Validate проверяет поля в порядке формы и возвращает нормализованные
// email и регион. Ошибка относится к первому неверному полю.
func (r Registration) Validate() (email string, region Region, err error) {
	if email, err = NormalizeEmail(r.Email); err != nil {
		return "", "", err
	}
	if err = ValidatePassword(r.Password); err != nil {
		return "", "", err
	}
	if r.Password != r.PasswordConfirm {
		return "", "", ErrPasswordMismatch
	}
	if region, err = ParseRegion(r.Region); err != nil {
		return "", "", err
	}
	if !r.Consent {
		return "", "", ErrConsentRequired
	}
	return email, region, nil
}

// LockedUntil применяет правило REQ-SEC-005 к последним неудачным попыткам
// входа, отсортированным от новых к старым. Если MaxLoginFailures попыток
// уложились в LoginWindow, вход блокируется на LoginLockout с момента
// последней из них. Попытки во время блокировки не записываются, поэтому
// блокировка не продлевается сама собой.
func LockedUntil(failures []time.Time, now time.Time) (time.Time, bool) {
	if len(failures) < MaxLoginFailures {
		return time.Time{}, false
	}
	newest, oldest := failures[0], failures[MaxLoginFailures-1]
	if newest.Sub(oldest) > LoginWindow {
		return time.Time{}, false
	}
	until := newest.Add(LoginLockout)
	return until, now.Before(until)
}

// Profile — данные профиля и настроек пользователя (GET /me).
type Profile struct {
	ID              int64     `json:"-"`
	Email           string    `json:"email"`
	Region          Region    `json:"region"`
	EmailVerified   bool      `json:"email_verified"`
	EmailEnabled    bool      `json:"email_enabled"`
	TelegramLinked  bool      `json:"telegram_linked"`
	TelegramEnabled bool      `json:"telegram_enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

// ProfileUpdate — частичное изменение профиля (PATCH /me). Nil означает
// «не менять». ConfirmRegionReset — согласие на сброс условий «цена ниже X»
// при смене региона (REQ-FUNC-005).
type ProfileUpdate struct {
	Region             *string
	EmailEnabled       *bool
	TelegramEnabled    *bool
	ConfirmRegionReset bool
}
