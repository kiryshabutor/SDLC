// Package httpapi — REST API /api/v1 и /healthz (DV-005).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/platform"
)

// WishlistService — сценарии UC-04, UC-05, UC-08 (реализация: app.Wishlist).
type WishlistService interface {
	Find(ctx context.Context, userID int64, query string) ([]domain.GameHit, error)
	Add(ctx context.Context, userID int64, ref string) (domain.WishlistItem, error)
	List(ctx context.Context, userID int64) ([]domain.WishlistItem, error)
}

// Health — данные для /healthz.
type Health interface {
	Ping(ctx context.Context) error
	LastCycleFinishedAt(ctx context.Context) (*time.Time, error)
}

// Server собирает обработчики.
type Server struct {
	Wishlist WishlistService
	Health   Health
	Log      *slog.Logger
	// UserID определяет пользователя запроса. В итерации 1 это временная
	// заглушка с демонстрационным пользователем; в F-02 её заменит сессия.
	UserID func(r *http.Request) (int64, bool)
}

// Handler возвращает корневой http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /api/v1/regions", s.regions)
	mux.HandleFunc("GET /api/v1/games/search", s.auth(s.search))
	mux.HandleFunc("GET /api/v1/wishlist", s.auth(s.listWishlist))
	mux.HandleFunc("POST /api/v1/wishlist", s.auth(s.addToWishlist))
	return s.logRequests(mux)
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorMap — соответствие доменных ошибок ответам (потоки ошибок UC-04, UC-05).
var errorMap = []struct {
	err    error
	status int
	body   apiError
}{
	{domain.ErrInvalidAppRef, http.StatusBadRequest, apiError{"invalid_app_ref", "Укажите ссылку на страницу игры в Steam или её appid", "ref"}},
	{domain.ErrQueryTooShort, http.StatusBadRequest, apiError{"query_too_short", "Введите не менее 2 символов", "q"}},
	{domain.ErrAlreadyInWishlist, http.StatusConflict, apiError{"already_in_wishlist", "Игра уже в вишлисте", ""}},
	{domain.ErrWishlistFull, http.StatusUnprocessableEntity, apiError{"wishlist_full", "Достигнут лимит вишлиста: 500 игр", ""}},
	{domain.ErrGameNotFound, http.StatusNotFound, apiError{"game_not_found", "Игра не найдена в Steam", "ref"}},
	{domain.ErrSourceUnavailable, http.StatusServiceUnavailable, apiError{"steam_unavailable", "Магазин Steam временно недоступен, попробуйте позже", ""}},
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	for _, m := range errorMap {
		if errors.Is(err, m.err) {
			writeJSON(w, m.status, map[string]apiError{"error": m.body})
			return
		}
	}
	s.Log.ErrorContext(r.Context(), "request failed", "request_id", platform.RequestID(r.Context()), "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]apiError{"error": {"internal", "Внутренняя ошибка сервера", ""}})
}

func (s *Server) auth(h func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := s.UserID(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]apiError{"error": {"unauthorized", "Войдите в систему", ""}})
			return
		}
		h(w, r, uid)
	}
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
	writeJSON(w, http.StatusOK, map[string]any{"regions": domain.Regions})
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
	items, err := s.Wishlist.List(r.Context(), uid)
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
