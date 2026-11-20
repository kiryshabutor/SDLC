package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kiryshabutor/SDLC/internal/app"
	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/i18n"
)

const goodSID = "good-sid"

// fakeAccounts принимает сессию goodSID и возвращает заданные ошибки.
type fakeAccounts struct {
	err        error
	affected   int
	loggedOut  string
	deleted    bool
	lastUpdate domain.ProfileUpdate
}

var profile = domain.Profile{ID: 1, Email: "kirill@example.com", Region: domain.RegionBY, EmailEnabled: true, TelegramEnabled: true}

func (f *fakeAccounts) Register(_ context.Context, r domain.Registration) (app.Session, error) {
	if f.err != nil {
		return app.Session{}, f.err
	}
	if _, _, err := r.Validate(); err != nil {
		return app.Session{}, err
	}
	return app.Session{Token: "new-sid", Profile: profile}, nil
}
func (f *fakeAccounts) Login(context.Context, string, string) (app.Session, error) {
	if f.err != nil {
		return app.Session{}, f.err
	}
	return app.Session{Token: "new-sid", Profile: profile}, nil
}
func (f *fakeAccounts) Logout(_ context.Context, token string) error {
	f.loggedOut = token
	return nil
}
func (f *fakeAccounts) Authenticate(_ context.Context, token string) (int64, bool, error) {
	return 1, token == goodSID, nil
}
func (f *fakeAccounts) Profile(context.Context, int64) (domain.Profile, error) { return profile, nil }
func (f *fakeAccounts) UpdateProfile(_ context.Context, _ int64, u domain.ProfileUpdate) (domain.Profile, int, error) {
	f.lastUpdate = u
	if f.err != nil {
		return domain.Profile{}, f.affected, f.err
	}
	p := profile
	if u.Region != nil {
		p.Region = domain.Region(*u.Region)
	}
	return p, 0, nil
}
func (f *fakeAccounts) DeleteAccount(_ context.Context, _ int64, password string) error {
	if password != "secret123" {
		return domain.ErrWrongPassword
	}
	f.deleted = true
	return nil
}

func cookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func anon(method, path, body string) *http.Request {
	return httptest.NewRequest(method, path, strings.NewReader(body))
}

// REQ-SEC-003: атрибуты cookie sid и csrf.
func TestRegisterSetsCookies(t *testing.T) {
	h := newServerWith(&fakeAccounts{}, fakeWishlist{}, fakeHealth{})
	rec, m := send(t, h, anon("POST", "/api/v1/auth/register",
		`{"email":"kirill@example.com","password":"secret123","password_confirm":"secret123","region":"BY","consent":true}`))
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%v", rec.Code, m)
	}
	sid := cookie(rec, "sid")
	if sid == nil || sid.Value != "new-sid" || !sid.HttpOnly || !sid.Secure || sid.SameSite != http.SameSiteLaxMode || sid.MaxAge != 30*24*3600 {
		t.Fatalf("sid = %+v", sid)
	}
	csrf := cookie(rec, "csrf")
	if csrf == nil || csrf.HttpOnly || !csrf.Secure || csrf.Value == "" || m["csrf_token"] != csrf.Value {
		t.Fatalf("csrf = %+v, body = %v", csrf, m)
	}
	if u, _ := m["user"].(map[string]any); u["email"] != "kirill@example.com" || u["region"] != "BY" {
		t.Fatalf("user = %v", m["user"])
	}
}

// REQ-FUNC-001: ошибка относится к конкретному полю формы.
func TestRegisterValidationFields(t *testing.T) {
	h := newServerWith(&fakeAccounts{}, fakeWishlist{}, fakeHealth{})
	cases := []struct{ body, code, field string }{
		{`{"email":"bad","password":"secret123","password_confirm":"secret123","region":"BY","consent":true}`, "invalid_email", "email"},
		{`{"email":"a@b.by","password":"short1","password_confirm":"short1","region":"BY","consent":true}`, "weak_password", "password"},
		{`{"email":"a@b.by","password":"secret123","password_confirm":"secret124","region":"BY","consent":true}`, "password_mismatch", "password_confirm"},
		{`{"email":"a@b.by","password":"secret123","password_confirm":"secret123","region":"DE","consent":true}`, "unknown_region", "region"},
		{`{"email":"a@b.by","password":"secret123","password_confirm":"secret123","region":"BY","consent":false}`, "consent_required", "consent"},
	}
	for _, tc := range cases {
		rec, m := send(t, h, anon("POST", "/api/v1/auth/register", tc.body))
		e, _ := m["error"].(map[string]any)
		if rec.Code != 400 || e["code"] != tc.code || e["field"] != tc.field || e["message"] == "" {
			t.Errorf("%s: code=%d body=%v", tc.code, rec.Code, m)
		}
	}
	rec, m := send(t, newServerWith(&fakeAccounts{err: domain.ErrEmailTaken}, fakeWishlist{}, fakeHealth{}),
		anon("POST", "/api/v1/auth/register", `{}`))
	if e, _ := m["error"].(map[string]any); rec.Code != 409 || e["field"] != "email" {
		t.Errorf("занятый email: code=%d body=%v", rec.Code, m)
	}
}

// REQ-FUNC-003, REQ-SEC-005: единое сообщение и блокировка.
func TestLoginErrors(t *testing.T) {
	cases := []struct {
		err    error
		status int
		msg    string
	}{
		{domain.ErrInvalidCredentials, 401, "Неверный email или пароль"},
		{domain.ErrLoginLocked, 429, "Слишком много неудачных попыток входа. Повторите через 15 минут"},
	}
	for _, tc := range cases {
		h := newServerWith(&fakeAccounts{err: tc.err}, fakeWishlist{}, fakeHealth{})
		rec, m := send(t, h, anon("POST", "/api/v1/auth/login", `{"email":"a@b.by","password":"x"}`))
		e, _ := m["error"].(map[string]any)
		if rec.Code != tc.status || e["message"] != tc.msg || cookie(rec, "sid") != nil {
			t.Errorf("%v: code=%d body=%v", tc.err, rec.Code, m)
		}
	}
	rec, _ := send(t, newServer(fakeWishlist{}, fakeHealth{}), anon("POST", "/api/v1/auth/login", `{"email":"a@b.by","password":"x"}`))
	if rec.Code != 200 || cookie(rec, "sid") == nil {
		t.Fatalf("успешный вход: code=%d", rec.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	h := newServer(fakeWishlist{}, fakeHealth{})
	for _, path := range []string{"/api/v1/wishlist", "/api/v1/me", "/api/v1/games/search?q=elden"} {
		req := anon("GET", path, "")
		req.AddCookie(&http.Cookie{Name: "sid", Value: "stale"})
		rec, m := send(t, h, req)
		if e, _ := m["error"].(map[string]any); rec.Code != 401 || e["code"] != "unauthorized" {
			t.Errorf("%s: code=%d body=%v", path, rec.Code, m)
		}
	}
}

// REQ-SEC-009: изменяющий запрос без X-CSRF-Token отклоняется.
func TestCSRF(t *testing.T) {
	h := newServer(fakeWishlist{}, fakeHealth{})
	for _, hdr := range []string{"", "wrong"} {
		req := anon("POST", "/api/v1/wishlist", `{"ref":"1245620"}`)
		req.AddCookie(&http.Cookie{Name: "sid", Value: goodSID})
		req.AddCookie(&http.Cookie{Name: "csrf", Value: "c1"})
		if hdr != "" {
			req.Header.Set("X-CSRF-Token", hdr)
		}
		rec, m := send(t, h, req)
		if e, _ := m["error"].(map[string]any); rec.Code != 403 || e["code"] != "csrf" {
			t.Errorf("header %q: code=%d body=%v", hdr, rec.Code, m)
		}
	}
	// GET не требует заголовка и выдаёт cookie csrf, если её не было.
	req := anon("GET", "/api/v1/me", "")
	req.AddCookie(&http.Cookie{Name: "sid", Value: goodSID})
	rec, _ := send(t, h, req)
	if rec.Code != 200 || cookie(rec, "csrf") == nil || cookie(rec, "sid").Value != goodSID {
		t.Fatalf("GET /me: code=%d cookies=%v", rec.Code, rec.Result().Cookies())
	}
}

func TestLogoutClearsCookies(t *testing.T) {
	acc := &fakeAccounts{}
	rec, _ := do(t, newServerWith(acc, fakeWishlist{}, fakeHealth{}), "POST", "/api/v1/auth/logout", "")
	if rec.Code != 204 || acc.loggedOut != goodSID {
		t.Fatalf("code=%d loggedOut=%q", rec.Code, acc.loggedOut)
	}
	if c := cookie(rec, "sid"); c == nil || c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("sid не удалена: %+v", c)
	}
}

// REQ-FUNC-005: смена региона, сбрасывающая условия, требует подтверждения.
func TestUpdateMeRegion(t *testing.T) {
	acc := &fakeAccounts{err: domain.ErrRegionResetRequired, affected: 2}
	rec, m := do(t, newServerWith(acc, fakeWishlist{}, fakeHealth{}), "PATCH", "/api/v1/me", `{"region":"PL"}`)
	e, _ := m["error"].(map[string]any)
	if rec.Code != 409 || e["code"] != "region_reset_required" || e["affected_conditions"] != 2.0 {
		t.Fatalf("code=%d body=%v", rec.Code, m)
	}
	acc = &fakeAccounts{}
	rec, m = do(t, newServerWith(acc, fakeWishlist{}, fakeHealth{}), "PATCH", "/api/v1/me",
		`{"region":"PL","confirm_region_reset":true,"email_enabled":false}`)
	if rec.Code != 200 || m["region"] != "PL" || !acc.lastUpdate.ConfirmRegionReset ||
		acc.lastUpdate.EmailEnabled == nil || *acc.lastUpdate.EmailEnabled || acc.lastUpdate.TelegramEnabled != nil {
		t.Fatalf("code=%d body=%v update=%+v", rec.Code, m, acc.lastUpdate)
	}
}

// REQ-FUNC-007: удаление только с верным паролем.
func TestDeleteMe(t *testing.T) {
	acc := &fakeAccounts{}
	h := newServerWith(acc, fakeWishlist{}, fakeHealth{})
	rec, m := do(t, h, "DELETE", "/api/v1/me", `{"password":"nope"}`)
	if e, _ := m["error"].(map[string]any); rec.Code != 403 || e["field"] != "password" || acc.deleted {
		t.Fatalf("неверный пароль: code=%d body=%v", rec.Code, m)
	}
	rec, _ = do(t, h, "DELETE", "/api/v1/me", `{"password":"secret123"}`)
	if rec.Code != 204 || !acc.deleted || cookie(rec, "sid").MaxAge >= 0 {
		t.Fatalf("code=%d deleted=%v", rec.Code, acc.deleted)
	}
}

// F-21: у каждого кода ошибки есть текст в файле локализации.
func TestErrorCodesLocalized(t *testing.T) {
	c := i18n.MustLoad(i18n.DefaultLang)
	codes := []string{"internal", "invalid_json", "unauthorized", "csrf"}
	for _, m := range errorMap {
		codes = append(codes, m.code)
	}
	for _, code := range codes {
		if !c.Has("error." + code) {
			t.Errorf("нет текста для error.%s в ru.json", code)
		}
	}
	for _, r := range domain.Regions {
		if !c.Has("region." + string(r.Code)) {
			t.Errorf("нет названия региона %s", r.Code)
		}
	}
}
