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

	// Подтверждение email, сброс пароля, Telegram (F-03, F-22).
	ErrTokenInvalid     = errors.New("token expired, used or unknown")
	ErrTooManyRequests  = errors.New("too many requests")
	ErrAlreadyVerified  = errors.New("email already verified")
	ErrTelegramDisabled = errors.New("telegram bot is not configured")

	// Вишлист и условия (F-06, F-07).
	ErrNotInWishlist         = errors.New("game not in wishlist")
	ErrInvalidCondition      = errors.New("unknown condition type")
	ErrInvalidConditionValue = errors.New("invalid condition value")
	ErrInvalidSort           = errors.New("unknown sort or filter")
)
