package app

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
	"github.com/kiryshabutor/SDLC/internal/i18n"
)

// AccountStore — операции хранилища для учётных записей и сессий.
type AccountStore interface {
	CreateUser(ctx context.Context, email, passwordHash string, region domain.Region, now time.Time, tok *domain.TokenSpec, msg *domain.Message) (int64, error)
	Credentials(ctx context.Context, email string) (int64, string, error)
	PasswordHash(ctx context.Context, userID int64) (string, error)
	RecentLoginFailures(ctx context.Context, email string, n int) ([]time.Time, error)
	RecordLoginFailure(ctx context.Context, email string, at time.Time) error
	ClearLoginFailures(ctx context.Context, email string) error
	CreateSession(ctx context.Context, idHash []byte, userID int64, now, expires time.Time) error
	TouchSession(ctx context.Context, idHash []byte, now, expires time.Time) (int64, error)
	DeleteSession(ctx context.Context, idHash []byte) error
	Profile(ctx context.Context, userID int64) (domain.Profile, error)
	CountConditions(ctx context.Context, userID int64, cond domain.Condition) (int, error)
	UpdateProfile(ctx context.Context, userID int64, region *domain.Region, emailEnabled, telegramEnabled *bool) error
	DeleteUser(ctx context.Context, userID int64) error

	IssueToken(ctx context.Context, userID int64, tok domain.TokenSpec, msg *domain.Message, now time.Time) error
	TokenStats(ctx context.Context, userID int64, purpose string, since time.Time) (*time.Time, int, error)
	VerifyEmail(ctx context.Context, hash []byte, now time.Time) (int64, error)
	ResetPassword(ctx context.Context, hash []byte, passwordHash string, now time.Time) (int64, error)
	UnlinkTelegram(ctx context.Context, userID int64) error
}

// Secrets — хэширование паролей и токенов (реализация: platform).
type Secrets struct {
	HashPassword  func(password string) (string, error)
	CheckPassword func(hash, password string) (bool, error)
	DummyHash     string
	NewToken      func() string
	HashToken     func(token string) []byte
}

// Accounts реализует UC-01 «Зарегистрироваться и подтвердить email»,
// UC-02 «Войти», UC-03 «Восстановить пароль», UC-11 «Привязать Telegram»
// и UC-15 «Изменить настройки или удалить аккаунт».
type Accounts struct {
	Store   AccountStore
	Secrets Secrets
	Now     func() time.Time
	// Texts — шаблоны писем (F-21); BaseURL — адрес веб-клиента для ссылок
	// в письмах; TelegramBot — имя бота без @, пусто — Telegram выключен.
	Texts       *i18n.Catalog
	BaseURL     string
	TelegramBot string
}

// Session — результат входа: значение cookie sid и профиль.
type Session struct {
	Token   string
	Profile domain.Profile
}

// Register создаёт учётную запись и сразу открывает сессию (DV-008).
func (a *Accounts) Register(ctx context.Context, r domain.Registration) (Session, error) {
	email, region, err := r.Validate()
	if err != nil {
		return Session{}, err
	}
	hash, err := a.Secrets.HashPassword(r.Password)
	if err != nil {
		return Session{}, err
	}
	now := a.now()
	tok, msg := a.mailToken(domain.PurposeVerify, domain.KindVerify, email, now)
	uid, err := a.Store.CreateUser(ctx, email, hash, region, now, &tok, &msg)
	if err != nil {
		return Session{}, err
	}
	return a.openSession(ctx, uid)
}

// mailToken создаёт токен и письмо со ссылкой на страницу веб-клиента
// /verify или /reset-password (REQ-FUNC-002, REQ-FUNC-004).
func (a *Accounts) mailToken(purpose, kind, email string, now time.Time) (domain.TokenSpec, domain.Message) {
	token := a.Secrets.NewToken()
	ttl, page := domain.VerifyTokenTTL, "/verify"
	if purpose == domain.PurposeReset {
		ttl, page = domain.ResetTokenTTL, "/reset-password"
	}
	link := strings.TrimRight(a.BaseURL, "/") + page + "?token=" + url.QueryEscape(token)
	params := map[string]string{"link": link}
	msg := domain.Message{Channel: domain.ChannelEmail, Kind: kind, Recipient: email,
		Subject: a.Texts.T("mail." + kind + ".subject"), Body: a.Texts.Format("mail."+kind+".body", params)}
	return domain.TokenSpec{Purpose: purpose, Hash: a.Secrets.HashToken(token), Expires: now.Add(ttl)}, msg
}

// mailAllowed применяет лимиты REQ-SEC-005 и REQ-FUNC-002: не больше
// 5 писем назначения в час и (для повторной отправки) не чаще раза в 60 с.
func (a *Accounts) mailAllowed(ctx context.Context, uid int64, purpose string, minInterval time.Duration, now time.Time) error {
	last, n, err := a.Store.TokenStats(ctx, uid, purpose, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	if n >= domain.MailRequestsPerHour || (last != nil && now.Sub(*last) < minInterval) {
		return domain.ErrTooManyRequests
	}
	return nil
}

// ResendVerification повторно отправляет письмо подтверждения; прежняя
// ссылка перестаёт действовать (REQ-FUNC-002).
func (a *Accounts) ResendVerification(ctx context.Context, userID int64) error {
	p, err := a.Store.Profile(ctx, userID)
	if err != nil {
		return err
	}
	if p.EmailVerified {
		return domain.ErrAlreadyVerified
	}
	now := a.now()
	if err := a.mailAllowed(ctx, userID, domain.PurposeVerify, domain.ResendInterval, now); err != nil {
		return err
	}
	tok, msg := a.mailToken(domain.PurposeVerify, domain.KindVerify, p.Email, now)
	msg.UserID = userID
	return a.Store.IssueToken(ctx, userID, tok, &msg, now)
}

// VerifyEmail подтверждает email по токену из письма.
func (a *Accounts) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return domain.ErrTokenInvalid
	}
	_, err := a.Store.VerifyEmail(ctx, a.Secrets.HashToken(token), a.now())
	return err
}

// RequestPasswordReset ставит письмо со ссылкой сброса. Ответ одинаков для
// зарегистрированного и незарегистрированного email и при превышении
// лимита (REQ-SEC-007): во всех этих случаях возвращается nil.
func (a *Accounts) RequestPasswordReset(ctx context.Context, emailInput string) error {
	email, err := domain.NormalizeEmail(emailInput)
	if err != nil {
		return err
	}
	uid, _, err := a.Store.Credentials(ctx, email)
	if errors.Is(err, domain.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	now := a.now()
	if err := a.mailAllowed(ctx, uid, domain.PurposeReset, 0, now); errors.Is(err, domain.ErrTooManyRequests) {
		return nil
	} else if err != nil {
		return err
	}
	tok, msg := a.mailToken(domain.PurposeReset, domain.KindReset, email, now)
	msg.UserID = uid
	return a.Store.IssueToken(ctx, uid, tok, &msg, now)
}

// ConfirmPasswordReset задаёт новый пароль по токену; все сессии
// пользователя завершаются (REQ-FUNC-004).
func (a *Accounts) ConfirmPasswordReset(ctx context.Context, token, password, confirm string) error {
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	if password != confirm {
		return domain.ErrPasswordMismatch
	}
	if token == "" {
		return domain.ErrTokenInvalid
	}
	hash, err := a.Secrets.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = a.Store.ResetPassword(ctx, a.Secrets.HashToken(token), hash, a.now())
	return err
}

// TelegramLink — код привязки и ссылка на бота (REQ-FUNC-031).
type TelegramLink struct {
	Code      string    `json:"code"`
	Link      string    `json:"link"`
	ExpiresAt time.Time `json:"expires_at"`
}

// LinkTelegram выдаёт одноразовый код на 15 минут и ссылку
// t.me/<бот>?start=<код>; прежний код перестаёт действовать.
func (a *Accounts) LinkTelegram(ctx context.Context, userID int64) (TelegramLink, error) {
	if a.TelegramBot == "" {
		return TelegramLink{}, domain.ErrTelegramDisabled
	}
	now := a.now()
	code := a.Secrets.NewToken()
	tok := domain.TokenSpec{Purpose: domain.PurposeTelegramLink, Hash: a.Secrets.HashToken(code), Expires: now.Add(domain.TelegramCodeTTL)}
	if err := a.Store.IssueToken(ctx, userID, tok, nil, now); err != nil {
		return TelegramLink{}, err
	}
	return TelegramLink{Code: code, Link: "https://t.me/" + a.TelegramBot + "?start=" + code, ExpiresAt: tok.Expires}, nil
}

// UnlinkTelegram отвязывает Telegram (кнопка в профиле).
func (a *Accounts) UnlinkTelegram(ctx context.Context, userID int64) error {
	return a.Store.UnlinkTelegram(ctx, userID)
}

// Login проверяет пару email/пароль с учётом блокировки перебора.
// Для любой неверной пары, в том числе для незарегистрированного email,
// возвращается одна и та же ошибка ErrInvalidCredentials (REQ-FUNC-003,
// REQ-SEC-007). Блокировка проверяется до пароля, поэтому во время неё
// отклоняется и верный пароль (REQ-SEC-005).
func (a *Accounts) Login(ctx context.Context, emailInput, password string) (Session, error) {
	email, err := domain.NormalizeEmail(emailInput)
	if err != nil {
		return Session{}, domain.ErrInvalidCredentials
	}
	now := a.now()
	failures, err := a.Store.RecentLoginFailures(ctx, email, domain.MaxLoginFailures)
	if err != nil {
		return Session{}, err
	}
	if _, locked := domain.LockedUntil(failures, now); locked {
		return Session{}, domain.ErrLoginLocked
	}

	uid, hash, err := a.Store.Credentials(ctx, email)
	found := err == nil
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return Session{}, err
	}
	if !found {
		hash = a.Secrets.DummyHash
	}
	ok, err := a.Secrets.CheckPassword(hash, password)
	if err != nil {
		return Session{}, err
	}
	if !ok || !found {
		if err := a.Store.RecordLoginFailure(ctx, email, now); err != nil {
			return Session{}, err
		}
		return Session{}, domain.ErrInvalidCredentials
	}
	if err := a.Store.ClearLoginFailures(ctx, email); err != nil {
		return Session{}, err
	}
	return a.openSession(ctx, uid)
}

// Logout завершает сессию. Неизвестный токен не считается ошибкой.
func (a *Accounts) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return a.Store.DeleteSession(ctx, a.Secrets.HashToken(token))
}

// Authenticate находит пользователя по cookie sid и продлевает сессию
// на 30 дней от текущего запроса (REQ-SEC-003).
func (a *Accounts) Authenticate(ctx context.Context, token string) (int64, bool, error) {
	if token == "" {
		return 0, false, nil
	}
	now := a.now()
	uid, err := a.Store.TouchSession(ctx, a.Secrets.HashToken(token), now, now.Add(domain.SessionIdleTTL))
	if errors.Is(err, domain.ErrUserNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return uid, true, nil
}

// Profile возвращает профиль пользователя.
func (a *Accounts) Profile(ctx context.Context, userID int64) (domain.Profile, error) {
	return a.Store.Profile(ctx, userID)
}

// UpdateProfile меняет регион и каналы уведомлений. Если смена региона
// сбросит условия «цена не выше X», без подтверждения возвращается
// ErrRegionResetRequired и число таких условий (REQ-FUNC-005).
func (a *Accounts) UpdateProfile(ctx context.Context, userID int64, u domain.ProfileUpdate) (domain.Profile, int, error) {
	var region *domain.Region
	if u.Region != nil {
		r, err := domain.ParseRegion(*u.Region)
		if err != nil {
			return domain.Profile{}, 0, err
		}
		cur, err := a.Store.Profile(ctx, userID)
		if err != nil {
			return domain.Profile{}, 0, err
		}
		if r != cur.Region {
			region = &r
			n, err := a.Store.CountConditions(ctx, userID, domain.ConditionBelow)
			if err != nil {
				return domain.Profile{}, 0, err
			}
			if n > 0 && !u.ConfirmRegionReset {
				return domain.Profile{}, n, domain.ErrRegionResetRequired
			}
		}
	}
	if err := a.Store.UpdateProfile(ctx, userID, region, u.EmailEnabled, u.TelegramEnabled); err != nil {
		return domain.Profile{}, 0, err
	}
	p, err := a.Store.Profile(ctx, userID)
	return p, 0, err
}

// DeleteAccount удаляет учётную запись после проверки текущего пароля
// (REQ-FUNC-007).
func (a *Accounts) DeleteAccount(ctx context.Context, userID int64, password string) error {
	hash, err := a.Store.PasswordHash(ctx, userID)
	if err != nil {
		return err
	}
	ok, err := a.Secrets.CheckPassword(hash, password)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWrongPassword
	}
	return a.Store.DeleteUser(ctx, userID)
}

func (a *Accounts) openSession(ctx context.Context, uid int64) (Session, error) {
	token := a.Secrets.NewToken()
	now := a.now()
	if err := a.Store.CreateSession(ctx, a.Secrets.HashToken(token), uid, now, now.Add(domain.SessionIdleTTL)); err != nil {
		return Session{}, err
	}
	p, err := a.Store.Profile(ctx, uid)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, Profile: p}, nil
}

func (a *Accounts) now() time.Time {
	if a.Now != nil {
		return a.Now().UTC().Truncate(time.Microsecond)
	}
	return time.Now().UTC().Truncate(time.Microsecond)
}
