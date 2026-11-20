package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/kiryshabutor/SDLC/internal/app"
	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/platform"
)

// Сессии и CSRF (ADR-0005, REQ-SEC-003, REQ-SEC-009).
const (
	sessionCookie = "sid"
	csrfCookie    = "csrf"
	csrfHeader    = "X-CSRF-Token"
)

var cookieMaxAge = int(domain.SessionIdleTTL.Seconds())

// setSessionCookies выдаёт cookie sid (HttpOnly) и csrf (читается SPA для
// заголовка X-CSRF-Token). Оба с Secure и SameSite=Lax.
func setSessionCookies(w http.ResponseWriter, sid, csrf string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: sid, Path: "/", MaxAge: cookieMaxAge,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	// csrf без HttpOnly: SPA читает её и повторяет в заголовке (double submit).
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: csrf, Path: "/", MaxAge: cookieMaxAge, //nolint:gosec // G124: намеренно без HttpOnly
		HttpOnly: false, Secure: true, SameSite: http.SameSiteLaxMode})
}

// clearSessionCookies удаляет cookie в браузере. Продлённые middleware auth
// cookie этого же ответа снимаются, иначе в ответе было бы два
// противоречащих Set-Cookie для sid.
func clearSessionCookies(w http.ResponseWriter) {
	w.Header().Del("Set-Cookie")
	for _, name := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, //nolint:gosec // G124: удаление, csrf без HttpOnly
			HttpOnly: name == sessionCookie, Secure: true, SameSite: http.SameSiteLaxMode})
	}
}

func cookieValue(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err == nil {
		return c.Value
	}
	return ""
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// auth пропускает запрос только с действующей сессией. Изменяющие запросы
// дополнительно требуют заголовок X-CSRF-Token, равный cookie csrf. Cookie
// перевыпускаются с новым Max-Age, чтобы срок в браузере совпадал со
// скользящим сроком сессии на сервере.
func (s *Server) auth(h func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid := cookieValue(r, sessionCookie)
		uid, ok, err := s.Accounts.Authenticate(r.Context(), sid)
		if err != nil {
			s.writeError(w, r, err)
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, s.errorBody("unauthorized", ""))
			return
		}
		csrf := cookieValue(r, csrfCookie)
		if !isSafeMethod(r.Method) {
			got := r.Header.Get(csrfHeader)
			if csrf == "" || subtle.ConstantTimeCompare([]byte(got), []byte(csrf)) != 1 {
				writeJSON(w, http.StatusForbidden, s.errorBody("csrf", ""))
				return
			}
		}
		if csrf == "" {
			csrf = platform.NewToken()
		}
		setSessionCookies(w, sid, csrf)
		h(w, r, uid)
	}
}

type sessionResponse struct {
	User      domain.Profile `json:"user"`
	CSRFToken string         `json:"csrf_token"`
}

func (s *Server) startSession(w http.ResponseWriter, status int, sess app.Session) {
	csrf := platform.NewToken()
	setSessionCookies(w, sess.Token, csrf)
	writeJSON(w, status, sessionResponse{User: sess.Profile, CSRFToken: csrf})
}

// register — POST /auth/register (REQ-FUNC-001).
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email           string `json:"email"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"password_confirm"`
		Region          string `json:"region"`
		Consent         bool   `json:"consent"`
	}
	if !s.decode(w, r, &req) {
		return
	}
	sess, err := s.Accounts.Register(r.Context(), domain.Registration(req))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.Log.InfoContext(r.Context(), "user registered", "request_id", platform.RequestID(r.Context()),
		"email", platform.MaskEmail(sess.Profile.Email), "region", sess.Profile.Region)
	s.startSession(w, http.StatusCreated, sess)
}

// login — POST /auth/login (REQ-FUNC-003, REQ-SEC-005, REQ-SEC-007).
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !s.decode(w, r, &req) {
		return
	}
	sess, err := s.Accounts.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrLoginLocked) || errors.Is(err, domain.ErrInvalidCredentials) {
			s.Log.WarnContext(r.Context(), "login rejected", "request_id", platform.RequestID(r.Context()),
				"email", platform.MaskEmail(req.Email), "locked", errors.Is(err, domain.ErrLoginLocked))
		}
		s.writeError(w, r, err)
		return
	}
	s.startSession(w, http.StatusOK, sess)
}

// logout — POST /auth/logout: сессия удаляется на сервере (REQ-SEC-003).
func (s *Server) logout(w http.ResponseWriter, r *http.Request, _ int64) {
	if err := s.Accounts.Logout(r.Context(), cookieValue(r, sessionCookie)); err != nil {
		s.writeError(w, r, err)
		return
	}
	clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

// me — GET /me.
func (s *Server) me(w http.ResponseWriter, r *http.Request, uid int64) {
	p, err := s.Accounts.Profile(r.Context(), uid)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// updateMe — PATCH /me: регион (REQ-FUNC-005) и каналы уведомлений
// (REQ-FUNC-006-2). Смена региона, которая сбросит условия «цена не выше X»,
// требует "confirm_region_reset": true, иначе 409 с числом таких условий.
func (s *Server) updateMe(w http.ResponseWriter, r *http.Request, uid int64) {
	var req struct {
		Region             *string `json:"region"`
		EmailEnabled       *bool   `json:"email_enabled"`
		TelegramEnabled    *bool   `json:"telegram_enabled"`
		ConfirmRegionReset bool    `json:"confirm_region_reset"`
	}
	if !s.decode(w, r, &req) {
		return
	}
	p, affected, err := s.Accounts.UpdateProfile(r.Context(), uid, domain.ProfileUpdate(req))
	if errors.Is(err, domain.ErrRegionResetRequired) {
		body := s.errorBody("region_reset_required", "region")
		e := body["error"]
		e.Affected = affected
		writeJSON(w, http.StatusConflict, map[string]apiError{"error": e})
		return
	}
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// deleteMe — DELETE /me с текущим паролем (REQ-FUNC-007).
func (s *Server) deleteMe(w http.ResponseWriter, r *http.Request, uid int64) {
	var req struct {
		Password string `json:"password"`
	}
	if !s.decode(w, r, &req) {
		return
	}
	if err := s.Accounts.DeleteAccount(r.Context(), uid, req.Password); err != nil {
		s.writeError(w, r, err)
		return
	}
	s.Log.InfoContext(r.Context(), "user deleted", "request_id", platform.RequestID(r.Context()), "user_id", uid)
	clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}
