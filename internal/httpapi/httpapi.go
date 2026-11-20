// Package httpapi — REST API /api/v1 и /healthz (DV-005).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/kiryshabutor/SDLC/internal/app"
	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/i18n"
	"github.com/kiryshabutor/SDLC/internal/platform"
)

// WishlistService — сценарии UC-04, UC-05, UC-07, UC-08 (реализация: app.Wishlist).
type WishlistService interface {
	Find(ctx context.Context, userID int64, query string) ([]domain.GameHit, error)
	Add(ctx context.Context, userID int64, ref string) (domain.WishlistItem, error)
	List(ctx context.Context, userID int64, q domain.WishlistQuery) ([]domain.WishlistItem, error)
	Remove(ctx context.Context, userID int64, appID domain.AppID) error
	SetCondition(ctx context.Context, userID int64, appID domain.AppID, typ string, value int) (domain.ItemCondition, error)
}

// AccountService — сценарии UC-01, UC-02, UC-15 (реализация: app.Accounts).
type AccountService interface {
	Register(ctx context.Context, r domain.Registration) (app.Session, error)
	Login(ctx context.Context, email, password string) (app.Session, error)
	Logout(ctx context.Context, token string) error
	Authenticate(ctx context.Context, token string) (int64, bool, error)
	Profile(ctx context.Context, userID int64) (domain.Profile, error)
	UpdateProfile(ctx context.Context, userID int64, u domain.ProfileUpdate) (domain.Profile, int, error)
	DeleteAccount(ctx context.Context, userID int64, password string) error

	ResendVerification(ctx context.Context, userID int64) error
	VerifyEmail(ctx context.Context, token string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, token, password, confirm string) error
	LinkTelegram(ctx context.Context, userID int64) (app.TelegramLink, error)
	UnlinkTelegram(ctx context.Context, userID int64) error
}

// Health — данные для /healthz.
type Health interface {
	Ping(ctx context.Context) error
	LastCycleFinishedAt(ctx context.Context) (*time.Time, error)
}

// Server собирает обработчики.
type Server struct {
	Accounts AccountService
	Wishlist WishlistService
	Health   Health
	Log      *slog.Logger
	Msg      *i18n.Catalog
}

// Handler возвращает корневой http.Handler.
func (s *Server) Handler() http.Handler {
	if s.Msg == nil {
		s.Msg = i18n.MustLoad(i18n.DefaultLang)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /api/v1/regions", s.regions)

	mux.HandleFunc("POST /api/v1/auth/register", s.register)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.auth(s.logout))
	mux.HandleFunc("POST /api/v1/auth/verify-email", s.verifyEmail)
	mux.HandleFunc("POST /api/v1/auth/resend-verification", s.auth(s.resendVerification))
	mux.HandleFunc("POST /api/v1/auth/password-reset", s.passwordReset)
	mux.HandleFunc("POST /api/v1/auth/password-reset/confirm", s.passwordResetConfirm)
	mux.HandleFunc("GET /api/v1/me", s.auth(s.me))
	mux.HandleFunc("PATCH /api/v1/me", s.auth(s.updateMe))
	mux.HandleFunc("DELETE /api/v1/me", s.auth(s.deleteMe))
	mux.HandleFunc("POST /api/v1/me/telegram/link", s.auth(s.telegramLink))
	mux.HandleFunc("DELETE /api/v1/me/telegram", s.auth(s.telegramUnlink))

	mux.HandleFunc("GET /api/v1/games/search", s.auth(s.search))
	mux.HandleFunc("GET /api/v1/wishlist", s.auth(s.listWishlist))
	mux.HandleFunc("POST /api/v1/wishlist", s.auth(s.addToWishlist))
	mux.HandleFunc("PATCH /api/v1/wishlist/{appid}", s.auth(s.setCondition))
	mux.HandleFunc("DELETE /api/v1/wishlist/{appid}", s.auth(s.removeFromWishlist))
	return s.logRequests(mux)
}

type apiError struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
	Affected int    `json:"affected_conditions,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorMap — соответствие доменных ошибок ответам. Текст сообщения берётся
// из файла локализации по ключу "error.<code>" (REQ-INT-001).
var errorMap = []struct {
	err    error
	status int
	code   string
	field  string
}{
	{domain.ErrInvalidAppRef, http.StatusBadRequest, "invalid_app_ref", "ref"},
	{domain.ErrQueryTooShort, http.StatusBadRequest, "query_too_short", "q"},
	{domain.ErrAlreadyInWishlist, http.StatusConflict, "already_in_wishlist", ""},
	{domain.ErrWishlistFull, http.StatusUnprocessableEntity, "wishlist_full", ""},
	{domain.ErrGameNotFound, http.StatusNotFound, "game_not_found", "ref"},
	{domain.ErrSourceUnavailable, http.StatusServiceUnavailable, "steam_unavailable", ""},

	{domain.ErrInvalidEmail, http.StatusBadRequest, "invalid_email", "email"},
	{domain.ErrWeakPassword, http.StatusBadRequest, "weak_password", "password"},
	{domain.ErrPasswordMismatch, http.StatusBadRequest, "password_mismatch", "password_confirm"},
	{domain.ErrUnknownRegion, http.StatusBadRequest, "unknown_region", "region"},
	{domain.ErrConsentRequired, http.StatusBadRequest, "consent_required", "consent"},
	{domain.ErrEmailTaken, http.StatusConflict, "email_taken", "email"},
	{domain.ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials", ""},
	{domain.ErrLoginLocked, http.StatusTooManyRequests, "login_locked", ""},
	{domain.ErrWrongPassword, http.StatusForbidden, "wrong_password", "password"},
	{domain.ErrRegionResetRequired, http.StatusConflict, "region_reset_required", "region"},
	{domain.ErrUserNotFound, http.StatusUnauthorized, "unauthorized", ""},

	{domain.ErrTokenInvalid, http.StatusGone, "token_invalid", "token"},
	{domain.ErrTooManyRequests, http.StatusTooManyRequests, "too_many_requests", ""},
	{domain.ErrAlreadyVerified, http.StatusConflict, "already_verified", ""},
	{domain.ErrTelegramDisabled, http.StatusServiceUnavailable, "telegram_disabled", ""},
	{domain.ErrNotInWishlist, http.StatusNotFound, "not_in_wishlist", ""},
	{domain.ErrInvalidCondition, http.StatusBadRequest, "invalid_condition", "condition.type"},
	{domain.ErrInvalidConditionValue, http.StatusBadRequest, "invalid_condition_value", "condition.value"},
	{domain.ErrInvalidSort, http.StatusBadRequest, "invalid_sort", "sort"},
}

func (s *Server) errorBody(code, field string) map[string]apiError {
	return map[string]apiError{"error": {Code: code, Message: s.Msg.T("error." + code), Field: field}}
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	for _, m := range errorMap {
		if errors.Is(err, m.err) {
			writeJSON(w, m.status, s.errorBody(m.code, m.field))
			return
		}
	}
	s.Log.ErrorContext(r.Context(), "request failed", "request_id", platform.RequestID(r.Context()), "err", err)
	writeJSON(w, http.StatusInternalServerError, s.errorBody("internal", ""))
}

// decode читает JSON-тело не больше 4 КиБ.
func (s *Server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, s.errorBody("invalid_json", ""))
		return false
	}
	return true
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	body := map[string]any{"status": "ok", "version": platform.Version, "last_cycle_at": nil}
	if err := s.Health.Ping(ctx); err != nil {
		body["status"] = "db_unavailable"
		writeJSON(w, http.StatusServiceUnavailable, body)
		return
	}
	if t, err := s.Health.LastCycleFinishedAt(ctx); err == nil && t != nil {
		body["last_cycle_at"] = t.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) regions(w http.ResponseWriter, _ *http.Request) {
	out := make([]domain.RegionInfo, len(domain.Regions))
	for i, r := range domain.Regions {
		r.Name = s.Msg.T("region." + string(r.Code))
		out[i] = r
	}
	writeJSON(w, http.StatusOK, map[string]any{"regions": out})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request, uid int64) {
	hits, err := s.Wishlist.Find(r.Context(), uid, r.URL.Query().Get("q"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if hits == nil {
		hits = []domain.GameHit{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": hits})
}

func (s *Server) listWishlist(w http.ResponseWriter, r *http.Request, uid int64) {
	q, err := domain.ParseWishlistQuery(r.URL.Query().Get("sort"), r.URL.Query()["filter"])
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	items, err := s.Wishlist.List(r.Context(), uid, q)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if items == nil {
		items = []domain.WishlistItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) addToWishlist(w http.ResponseWriter, r *http.Request, uid int64) {
	var req struct {
		Ref string `json:"ref"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, r, domain.ErrInvalidAppRef)
		return
	}
	item, err := s.Wishlist.Add(r.Context(), uid, req.Ref)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// pathAppID читает {appid} из пути; некорректное значение даёт 404.
func pathAppID(r *http.Request) (domain.AppID, error) {
	id, err := strconv.Atoi(r.PathValue("appid"))
	if err != nil || id <= 0 {
		return 0, domain.ErrNotInWishlist
	}
	return domain.AppID(id), nil
}

// removeFromWishlist — DELETE /wishlist/{appid} (REQ-FUNC-014).
func (s *Server) removeFromWishlist(w http.ResponseWriter, r *http.Request, uid int64) {
	id, err := pathAppID(r)
	if err == nil {
		err = s.Wishlist.Remove(r.Context(), uid, id)
	}
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setCondition — PATCH /wishlist/{appid} {"condition":{"type","value"}}
// (REQ-FUNC-015). Значение для below — в минимальных единицах валюты.
func (s *Server) setCondition(w http.ResponseWriter, r *http.Request, uid int64) {
	id, err := pathAppID(r)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	var req struct {
		Condition struct {
			Type  string `json:"type"`
			Value int    `json:"value"`
		} `json:"condition"`
	}
	if !s.decode(w, r, &req) {
		return
	}
	c, err := s.Wishlist.SetCondition(r.Context(), uid, id, req.Condition.Type, req.Condition.Value)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"appid": id, "condition": c})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// logRequests пишет строку журнала на каждый запрос с request_id (REQ-OBS-001).
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 64 {
			id = platform.NewRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(platform.WithRequestID(r.Context(), id))
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		if r.URL.Path == "/healthz" && sw.status == http.StatusOK {
			return
		}
		s.Log.InfoContext(r.Context(), "http request", "request_id", id, "method", r.Method,
			"path", r.URL.Path, "status", sw.status, "duration_ms", time.Since(start).Milliseconds())
	})
}
