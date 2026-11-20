package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// memAccounts — хранилище учётных записей в памяти.
type memAccounts struct {
	users    map[int64]*memUser
	byEmail  map[string]int64
	failures map[string][]time.Time // от старых к новым
	sessions map[string]memSession
	conds    map[int64]map[domain.Condition]int
	nextID   int64
}

type memUser struct {
	email, hash string
	region      domain.Region
	emailOn     bool
}

type memSession struct {
	uid     int64
	expires time.Time
}

func newMemAccounts() *memAccounts {
	return &memAccounts{users: map[int64]*memUser{}, byEmail: map[string]int64{}, failures: map[string][]time.Time{},
		sessions: map[string]memSession{}, conds: map[int64]map[domain.Condition]int{}}
}

func (m *memAccounts) CreateUser(_ context.Context, email, hash string, r domain.Region, _ time.Time) (int64, error) {
	if _, ok := m.byEmail[email]; ok {
		return 0, domain.ErrEmailTaken
	}
	m.nextID++
	m.users[m.nextID] = &memUser{email: email, hash: hash, region: r, emailOn: true}
	m.byEmail[email] = m.nextID
	return m.nextID, nil
}
func (m *memAccounts) Credentials(_ context.Context, email string) (int64, string, error) {
	id, ok := m.byEmail[email]
	if !ok {
		return 0, "", domain.ErrUserNotFound
	}
	return id, m.users[id].hash, nil
}
func (m *memAccounts) PasswordHash(_ context.Context, id int64) (string, error) {
	u, ok := m.users[id]
	if !ok {
		return "", domain.ErrUserNotFound
	}
	return u.hash, nil
}
func (m *memAccounts) RecentLoginFailures(_ context.Context, email string, n int) ([]time.Time, error) {
	f := m.failures[email]
	var out []time.Time
	for i := len(f) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, f[i])
	}
	return out, nil
}
func (m *memAccounts) RecordLoginFailure(_ context.Context, email string, at time.Time) error {
	m.failures[email] = append(m.failures[email], at)
	return nil
}
func (m *memAccounts) ClearLoginFailures(_ context.Context, email string) error {
	delete(m.failures, email)
	return nil
}
func (m *memAccounts) CreateSession(_ context.Context, h []byte, uid int64, _, exp time.Time) error {
	m.sessions[string(h)] = memSession{uid, exp}
	return nil
}
func (m *memAccounts) TouchSession(_ context.Context, h []byte, now, exp time.Time) (int64, error) {
	s, ok := m.sessions[string(h)]
	if !ok || !now.Before(s.expires) {
		return 0, domain.ErrUserNotFound
	}
	m.sessions[string(h)] = memSession{s.uid, exp}
	return s.uid, nil
}
func (m *memAccounts) DeleteSession(_ context.Context, h []byte) error {
	delete(m.sessions, string(h))
	return nil
}
func (m *memAccounts) Profile(_ context.Context, id int64) (domain.Profile, error) {
	u, ok := m.users[id]
	if !ok {
		return domain.Profile{}, domain.ErrUserNotFound
	}
	return domain.Profile{ID: id, Email: u.email, Region: u.region, EmailEnabled: u.emailOn}, nil
}
func (m *memAccounts) CountConditions(_ context.Context, id int64, c domain.Condition) (int, error) {
	return m.conds[id][c], nil
}
func (m *memAccounts) UpdateProfile(_ context.Context, id int64, r *domain.Region, emailOn, _ *bool) error {
	u := m.users[id]
	if r != nil {
		u.region = *r
		if n := m.conds[id][domain.ConditionBelow]; n > 0 {
			m.conds[id][domain.ConditionAny] += n
			m.conds[id][domain.ConditionBelow] = 0
		}
	}
	if emailOn != nil {
		u.emailOn = *emailOn
	}
	return nil
}
func (m *memAccounts) DeleteUser(_ context.Context, id int64) error {
	u, ok := m.users[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	delete(m.byEmail, u.email)
	delete(m.users, id)
	for k, s := range m.sessions {
		if s.uid == id {
			delete(m.sessions, k)
		}
	}
	return nil
}

// fastSecrets — быстрые заменители Argon2id для тестов сценариев;
// настоящий алгоритм проверяется в platform.
func fastSecrets() Secrets {
	n := 0
	return Secrets{
		HashPassword:  func(p string) (string, error) { return "h:" + p, nil },
		CheckPassword: func(h, p string) (bool, error) { return h == "h:"+p, nil },
		DummyHash:     "h:\x00dummy",
		NewToken:      func() string { n++; return "tok" + string(rune('a'+n)) },
		HashToken:     func(t string) []byte { return []byte("sha:" + t) },
	}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newAccounts() (*Accounts, *memAccounts, *clock) {
	st := newMemAccounts()
	c := &clock{time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}
	return &Accounts{Store: st, Secrets: fastSecrets(), Now: c.now}, st, c
}

var reg = domain.Registration{Email: " Kirill@Example.com ", Password: "secret123", PasswordConfirm: "secret123", Region: "BY", Consent: true}

func TestRegisterAndAuthenticate(t *testing.T) {
	a, st, c := newAccounts()
	ctx := context.Background()
	s, err := a.Register(ctx, reg)
	if err != nil || s.Token == "" || s.Profile.Email != "kirill@example.com" || s.Profile.Region != domain.RegionBY {
		t.Fatalf("session=%+v err=%v", s, err)
	}
	if st.users[1].hash != "h:secret123" {
		t.Fatal("пароль должен храниться только в виде хэша")
	}
	if _, err := a.Register(ctx, reg); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("повторная регистрация: %v", err)
	}
	bad := reg
	bad.Consent = false
	if _, err := a.Register(ctx, bad); !errors.Is(err, domain.ErrConsentRequired) {
		t.Fatalf("без согласия: %v", err)
	}

	uid, ok, err := a.Authenticate(ctx, s.Token)
	if err != nil || !ok || uid != 1 {
		t.Fatalf("auth: %d %v %v", uid, ok, err)
	}
	// Скользящий срок: активность через 29 дней продлевает сессию ещё на 30.
	c.t = c.t.Add(29 * 24 * time.Hour)
	if _, ok, _ := a.Authenticate(ctx, s.Token); !ok {
		t.Fatal("сессия должна продлеваться при активности")
	}
	c.t = c.t.Add(29 * 24 * time.Hour)
	if _, ok, _ := a.Authenticate(ctx, s.Token); !ok {
		t.Fatal("сессия истекла раньше 30 дней неактивности")
	}
	c.t = c.t.Add(30*24*time.Hour + time.Second)
	if _, ok, _ := a.Authenticate(ctx, s.Token); ok {
		t.Fatal("сессия должна истечь после 30 дней неактивности")
	}
	if _, ok, _ := a.Authenticate(ctx, ""); ok {
		t.Fatal("пустой токен принят")
	}
}

func TestLoginLogout(t *testing.T) {
	a, _, _ := newAccounts()
	ctx := context.Background()
	if _, err := a.Register(ctx, reg); err != nil {
		t.Fatal(err)
	}
	s, err := a.Login(ctx, "KIRILL@example.com", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	// Неверный пароль и неизвестный email неразличимы (REQ-SEC-007).
	_, e1 := a.Login(ctx, "kirill@example.com", "wrong123")
	_, e2 := a.Login(ctx, "nobody@example.com", "secret123")
	_, e3 := a.Login(ctx, "not-an-email", "secret123")
	for _, e := range []error{e1, e2, e3} {
		if !errors.Is(e, domain.ErrInvalidCredentials) {
			t.Fatalf("err = %v", e)
		}
	}
	if err := a.Logout(ctx, s.Token); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := a.Authenticate(ctx, s.Token); ok {
		t.Fatal("после выхода сессия должна быть недействительна (REQ-SEC-003)")
	}
}

// REQ-SEC-005: 6-я попытка за 15 минут отклоняется даже с верным паролем.
func TestLoginLockout(t *testing.T) {
	a, st, c := newAccounts()
	ctx := context.Background()
	if _, err := a.Register(ctx, reg); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := a.Login(ctx, "kirill@example.com", "wrong123"); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("попытка %d: %v", i+1, err)
		}
		c.t = c.t.Add(time.Minute)
	}
	if _, err := a.Login(ctx, "kirill@example.com", "secret123"); !errors.Is(err, domain.ErrLoginLocked) {
		t.Fatalf("6-я попытка: %v", err)
	}
	if n := len(st.failures["kirill@example.com"]); n != 5 {
		t.Fatalf("попытки во время блокировки не записываются: %d", n)
	}
	c.t = c.t.Add(15 * time.Minute)
	if _, err := a.Login(ctx, "kirill@example.com", "secret123"); err != nil {
		t.Fatalf("после блокировки: %v", err)
	}
	if len(st.failures["kirill@example.com"]) != 0 {
		t.Fatal("успешный вход должен очищать счётчик")
	}
	// Блокировка работает и для незарегистрированного адреса.
	for i := 0; i < 5; i++ {
		_, _ = a.Login(ctx, "ghost@example.com", "x")
	}
	if _, err := a.Login(ctx, "ghost@example.com", "x"); !errors.Is(err, domain.ErrLoginLocked) {
		t.Fatalf("ghost: %v", err)
	}
}

func TestUpdateProfileRegion(t *testing.T) {
	a, st, _ := newAccounts()
	ctx := context.Background()
	s, _ := a.Register(ctx, reg)
	uid := s.Profile.ID
	st.conds[uid] = map[domain.Condition]int{domain.ConditionBelow: 2, domain.ConditionMinPct: 1}

	pl, off := "PL", false
	if _, n, err := a.UpdateProfile(ctx, uid, domain.ProfileUpdate{Region: &pl}); !errors.Is(err, domain.ErrRegionResetRequired) || n != 2 {
		t.Fatalf("без подтверждения: n=%d err=%v", n, err)
	}
	if st.users[uid].region != domain.RegionBY {
		t.Fatal("регион не должен меняться без подтверждения")
	}
	p, _, err := a.UpdateProfile(ctx, uid, domain.ProfileUpdate{Region: &pl, ConfirmRegionReset: true, EmailEnabled: &off})
	if err != nil || p.Region != domain.RegionPL || p.EmailEnabled {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if st.conds[uid][domain.ConditionBelow] != 0 || st.conds[uid][domain.ConditionMinPct] != 1 {
		t.Fatalf("условия: %v", st.conds[uid])
	}
	// Тот же регион и регион без условий «ниже X» подтверждения не требуют.
	if _, _, err := a.UpdateProfile(ctx, uid, domain.ProfileUpdate{Region: &pl}); err != nil {
		t.Fatal(err)
	}
	us := "US"
	if p, _, err := a.UpdateProfile(ctx, uid, domain.ProfileUpdate{Region: &us}); err != nil || p.Region != domain.RegionUS {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	de := "DE"
	if _, _, err := a.UpdateProfile(ctx, uid, domain.ProfileUpdate{Region: &de}); !errors.Is(err, domain.ErrUnknownRegion) {
		t.Fatalf("DE: %v", err)
	}
}

func TestDeleteAccount(t *testing.T) {
	a, st, _ := newAccounts()
	ctx := context.Background()
	s, _ := a.Register(ctx, reg)
	if err := a.DeleteAccount(ctx, s.Profile.ID, "wrong123"); !errors.Is(err, domain.ErrWrongPassword) {
		t.Fatalf("неверный пароль: %v", err)
	}
	if err := a.DeleteAccount(ctx, s.Profile.ID, "secret123"); err != nil {
		t.Fatal(err)
	}
	if len(st.users) != 0 || len(st.sessions) != 0 {
		t.Fatal("данные пользователя не удалены")
	}
	if _, err := a.Login(ctx, "kirill@example.com", "secret123"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("вход после удаления: %v", err)
	}
}
