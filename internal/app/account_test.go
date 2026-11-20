package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/i18n"
)

// memAccounts — хранилище учётных записей в памяти.
type memAccounts struct {
	users    map[int64]*memUser
	byEmail  map[string]int64
	failures map[string][]time.Time // от старых к новым
	sessions map[string]memSession
	conds    map[int64]map[domain.Condition]int
	tokens   map[string]*memToken
	outbox   []domain.Message
	nextID   int64
}

type memToken struct {
	uid              int64
	purpose          string
	created, expires time.Time
	used             bool
}

type memUser struct {
	email, hash string
	region      domain.Region
	emailOn     bool
	verified    bool
	telegram    bool
}

type memSession struct {
	uid     int64
	expires time.Time
}

func newMemAccounts() *memAccounts {
	return &memAccounts{users: map[int64]*memUser{}, byEmail: map[string]int64{}, failures: map[string][]time.Time{},
		sessions: map[string]memSession{}, conds: map[int64]map[domain.Condition]int{}, tokens: map[string]*memToken{}}
}

func (m *memAccounts) CreateUser(ctx context.Context, email, hash string, r domain.Region, now time.Time, tok *domain.TokenSpec, msg *domain.Message) (int64, error) {
	if _, ok := m.byEmail[email]; ok {
		return 0, domain.ErrEmailTaken
	}
	m.nextID++
	m.users[m.nextID] = &memUser{email: email, hash: hash, region: r, emailOn: true}
	m.byEmail[email] = m.nextID
	if tok != nil {
		_ = m.IssueToken(ctx, m.nextID, *tok, msg, now)
	}
	return m.nextID, nil
}
func (m *memAccounts) IssueToken(_ context.Context, uid int64, tok domain.TokenSpec, msg *domain.Message, now time.Time) error {
	for _, t := range m.tokens {
		if t.uid == uid && t.purpose == tok.Purpose {
			t.used = true
		}
	}
	m.tokens[string(tok.Hash)] = &memToken{uid: uid, purpose: tok.Purpose, created: now, expires: tok.Expires}
	if msg != nil {
		m.outbox = append(m.outbox, *msg)
	}
	return nil
}
func (m *memAccounts) TokenStats(_ context.Context, uid int64, purpose string, since time.Time) (*time.Time, int, error) {
	var last *time.Time
	n := 0
	for _, t := range m.tokens {
		if t.uid != uid || t.purpose != purpose {
			continue
		}
		if last == nil || t.created.After(*last) {
			c := t.created
			last = &c
		}
		if !t.created.Before(since) {
			n++
		}
	}
	return last, n, nil
}
func (m *memAccounts) use(hash []byte, purpose string, now time.Time) (int64, error) {
	t, ok := m.tokens[string(hash)]
	if !ok || t.used || t.purpose != purpose || !now.Before(t.expires) {
		return 0, domain.ErrTokenInvalid
	}
	t.used = true
	return t.uid, nil
}
func (m *memAccounts) VerifyEmail(_ context.Context, hash []byte, now time.Time) (int64, error) {
	uid, err := m.use(hash, domain.PurposeVerify, now)
	if err == nil {
		m.users[uid].verified = true
	}
	return uid, err
}
func (m *memAccounts) ResetPassword(_ context.Context, hash []byte, pw string, now time.Time) (int64, error) {
	uid, err := m.use(hash, domain.PurposeReset, now)
	if err != nil {
		return 0, err
	}
	m.users[uid].hash = pw
	for k, s := range m.sessions {
		if s.uid == uid {
			delete(m.sessions, k)
		}
	}
	return uid, nil
}
func (m *memAccounts) UnlinkTelegram(_ context.Context, uid int64) error {
	m.users[uid].telegram = false
	return nil
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
	return domain.Profile{ID: id, Email: u.email, Region: u.region, EmailEnabled: u.emailOn, EmailVerified: u.verified, TelegramLinked: u.telegram}, nil
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
		NewToken:      func() string { n++; return fmt.Sprintf("tok%d", n) },
		HashToken:     func(t string) []byte { return []byte("sha:" + t) },
	}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newAccounts() (*Accounts, *memAccounts, *clock) {
	st := newMemAccounts()
	c := &clock{time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}
	return &Accounts{Store: st, Secrets: fastSecrets(), Now: c.now, Texts: i18n.MustLoad(i18n.DefaultLang),
		BaseURL: "https://deals.example/"}, st, c
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

// tokenFromOutbox достаёт токен из ссылки последнего письма.
func tokenFromOutbox(t *testing.T, st *memAccounts) string {
	t.Helper()
	if len(st.outbox) == 0 {
		t.Fatal("outbox пуст")
	}
	body := st.outbox[len(st.outbox)-1].Body
	i := strings.Index(body, "token=")
	if i < 0 {
		t.Fatalf("нет ссылки в письме: %q", body)
	}
	return strings.Fields(body[i+len("token="):])[0]
}

// REQ-FUNC-002: письмо при регистрации, ссылка на 24 часа, однократно.
func TestVerifyEmail(t *testing.T) {
	a, st, c := newAccounts()
	ctx := context.Background()
	s, _ := a.Register(ctx, reg)
	if len(st.outbox) != 1 || st.outbox[0].Recipient != "kirill@example.com" || st.outbox[0].Kind != domain.KindVerify ||
		!strings.Contains(st.outbox[0].Body, "https://deals.example/verify?token=") || st.outbox[0].Subject == "" {
		t.Fatalf("outbox = %+v", st.outbox)
	}
	tok := tokenFromOutbox(t, st)
	if err := a.VerifyEmail(ctx, "nope"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("чужой токен: %v", err)
	}
	if err := a.VerifyEmail(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if p, _ := a.Profile(ctx, s.Profile.ID); !p.EmailVerified {
		t.Fatal("email не подтверждён")
	}
	if err := a.VerifyEmail(ctx, tok); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("повторное использование: %v", err)
	}
	if err := a.ResendVerification(ctx, s.Profile.ID); !errors.Is(err, domain.ErrAlreadyVerified) {
		t.Fatalf("после подтверждения: %v", err)
	}

	// Истечение через 24 часа.
	reg2 := reg
	reg2.Email = "late@example.com"
	s2, _ := a.Register(ctx, reg2)
	tok2 := tokenFromOutbox(t, st)
	c.t = c.t.Add(24*time.Hour + time.Second)
	if err := a.VerifyEmail(ctx, tok2); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("истёкший: %v", err)
	}
	_ = s2
}

// REQ-FUNC-002, REQ-SEC-005: повтор не чаще раза в 60 с и 5 раз в час;
// прежняя ссылка перестаёт действовать.
func TestResendVerification(t *testing.T) {
	a, st, c := newAccounts()
	ctx := context.Background()
	s, _ := a.Register(ctx, reg)
	first := tokenFromOutbox(t, st)
	if err := a.ResendVerification(ctx, s.Profile.ID); !errors.Is(err, domain.ErrTooManyRequests) {
		t.Fatalf("сразу после регистрации: %v", err)
	}
	for i := 0; i < 4; i++ {
		c.t = c.t.Add(61 * time.Second)
		if err := a.ResendVerification(ctx, s.Profile.ID); err != nil {
			t.Fatalf("повтор %d: %v", i+1, err)
		}
	}
	c.t = c.t.Add(61 * time.Second)
	if err := a.ResendVerification(ctx, s.Profile.ID); !errors.Is(err, domain.ErrTooManyRequests) {
		t.Fatalf("6-е письмо за час: %v", err)
	}
	if len(st.outbox) != 5 {
		t.Fatalf("писем %d", len(st.outbox))
	}
	if err := a.VerifyEmail(ctx, first); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("старая ссылка должна перестать действовать: %v", err)
	}
	if err := a.VerifyEmail(ctx, tokenFromOutbox(t, st)); err != nil {
		t.Fatal(err)
	}
}

// REQ-FUNC-004, REQ-SEC-007: одинаковый ответ, однократная ссылка на час,
// после сброса все сессии завершены, старый пароль не действует.
func TestPasswordReset(t *testing.T) {
	a, st, c := newAccounts()
	ctx := context.Background()
	s, _ := a.Register(ctx, reg)
	before := len(st.outbox)
	if err := a.RequestPasswordReset(ctx, "ghost@example.com"); err != nil {
		t.Fatalf("незарегистрированный: %v", err)
	}
	if len(st.outbox) != before {
		t.Fatal("письмо на незарегистрированный адрес")
	}
	if err := a.RequestPasswordReset(ctx, "bad"); !errors.Is(err, domain.ErrInvalidEmail) {
		t.Fatalf("формат: %v", err)
	}
	if err := a.RequestPasswordReset(ctx, "KIRILL@example.com"); err != nil {
		t.Fatal(err)
	}
	msg := st.outbox[len(st.outbox)-1]
	if msg.Kind != domain.KindReset || !strings.Contains(msg.Body, "/reset-password?token=") {
		t.Fatalf("письмо: %+v", msg)
	}
	tok := tokenFromOutbox(t, st)
	if err := a.ConfirmPasswordReset(ctx, tok, "short", "short"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Fatalf("слабый: %v", err)
	}
	if err := a.ConfirmPasswordReset(ctx, tok, "newpass123", "newpass124"); !errors.Is(err, domain.ErrPasswordMismatch) {
		t.Fatalf("несовпадение: %v", err)
	}
	if err := a.ConfirmPasswordReset(ctx, tok, "newpass123", "newpass123"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := a.Authenticate(ctx, s.Token); ok {
		t.Fatal("сессии должны завершиться")
	}
	if _, err := a.Login(ctx, "kirill@example.com", "secret123"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatal("старый пароль действует")
	}
	if _, err := a.Login(ctx, "kirill@example.com", "newpass123"); err != nil {
		t.Fatal(err)
	}
	if err := a.ConfirmPasswordReset(ctx, tok, "other123", "other123"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("повторное использование: %v", err)
	}
	// Ссылка действует 1 час.
	_ = a.RequestPasswordReset(ctx, "kirill@example.com")
	tok = tokenFromOutbox(t, st)
	c.t = c.t.Add(time.Hour + time.Second)
	if err := a.ConfirmPasswordReset(ctx, tok, "other123", "other123"); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Fatalf("истёкшая: %v", err)
	}
	// Лимит 5 писем в час: шестой запрос молча не отправляет письмо.
	for i := 0; i < 6; i++ {
		_ = a.RequestPasswordReset(ctx, "kirill@example.com")
	}
	n := 0
	for _, m := range st.outbox {
		if m.Kind == domain.KindReset {
			n++
		}
	}
	if n != 2+5 { // два письма из первого часа и пять в текущем
		t.Fatalf("писем сброса: %d", n)
	}
}

func TestLinkTelegram(t *testing.T) {
	a, st, _ := newAccounts()
	ctx := context.Background()
	s, _ := a.Register(ctx, reg)
	if _, err := a.LinkTelegram(ctx, s.Profile.ID); !errors.Is(err, domain.ErrTelegramDisabled) {
		t.Fatalf("без бота: %v", err)
	}
	a.TelegramBot = "deal_hunter_bot"
	l, err := a.LinkTelegram(ctx, s.Profile.ID)
	if err != nil || l.Link != "https://t.me/deal_hunter_bot?start="+l.Code || !l.ExpiresAt.Equal(a.now().Add(15*time.Minute)) {
		t.Fatalf("%+v %v", l, err)
	}
	if tk := st.tokens["sha:"+l.Code]; tk == nil || tk.purpose != domain.PurposeTelegramLink {
		t.Fatal("код не сохранён")
	}
	st.users[s.Profile.ID].telegram = true
	if err := a.UnlinkTelegram(ctx, s.Profile.ID); err != nil || st.users[s.Profile.ID].telegram {
		t.Fatal("отвязка")
	}
}
