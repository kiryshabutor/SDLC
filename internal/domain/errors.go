package domain

import "errors"

// Ошибки предметной области. Слой httpapi отображает их в коды ответа.
var (
	ErrInvalidAppRef     = errors.New("invalid app reference")
	ErrUnknownRegion     = errors.New("unknown region")
	ErrGameNotFound      = errors.New("game not found in Steam")
	ErrAlreadyInWishlist = errors.New("game already in wishlist")
	ErrWishlistFull      = errors.New("wishlist limit reached")
	ErrQueryTooShort     = errors.New("search query too short")
	ErrSourceUnavailable = errors.New("price source unavailable")
)
