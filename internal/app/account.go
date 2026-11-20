package app

import (
	"context"
	"errors"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// AccountStore — операции хранилища для учётных записей и сессий.
type AccountStore interface {
	CreateUser(ctx context.Context, email, passwordHash string, region domain.Region, consentAt time.Time) (int64, error)
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
}

// Secrets — хэширование паролей и токенов (реализация: platform).
type Secrets struct {
	HashPassword  func(password string) (string, error)
	CheckPassword func(hash, password string) (bool, error)
	DummyHash     string
	NewToken      func() string
	HashToken     func(token string) []byte
}

// Accounts реализует UC-01 «Зарегистрироваться», UC-02 «Войти» и UC-15
// «Изменить настройки или удалить аккаунт». Подтверждение email и сброс
// пароля (UC-01 шаг 6, UC-03) добавляются в F-03.
type Accounts struct {
	Store   AccountStore
	Secrets Secrets
	Now     func() time.Time
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
	uid, err := a.Store.CreateUser(ctx, email, hash, region, a.now())
	if err != nil {
		return Session{}, err
	}
	return a.openSession(ctx, uid)
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
