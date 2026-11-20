package domain

import "errors"

// Ошибки предметной области. Слой httpapi отображает их в коды ответа,
// тексты сообщений берутся из файла локализации (internal/i18n).
var (
	ErrInvalidAppRef     = errors.New("invalid app reference")
	ErrUnknownRegion     = errors.New("unknown region")
	ErrGameNotFound      = errors.New("game not found in Steam")
	ErrAlreadyInWishlist = errors.New("game already in wishlist")
	ErrWishlistFull      = errors.New("wishlist limit reached")
	ErrQueryTooShort     = errors.New("search query too short")
	ErrSourceUnavailable = errors.New("price source unavailable")

	// Учётная запись (F-02, F-04, F-17).
	ErrInvalidEmail        = errors.New("invalid email")
	ErrWeakPassword        = errors.New("password does not meet requirements")
	ErrPasswordMismatch    = errors.New("passwords do not match")
	ErrConsentRequired     = errors.New("personal data consent required")
	ErrEmailTaken          = errors.New("email already registered")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrLoginLocked         = errors.New("login temporarily locked")
	ErrWrongPassword       = errors.New("wrong current password")
	ErrRegionResetRequired = errors.New("region change resets price conditions")
	ErrUserNotFound        = errors.New("user not found")
)
