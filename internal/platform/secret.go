package platform

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Параметры Argon2id — минимальная рекомендация OWASP (19 МиБ, 2 прохода,
// 1 поток): около 30 мс на хэш на сервере DV-014 (REQ-SEC-001).
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

var errBadHash = errors.New("malformed password hash")

// HashPassword возвращает хэш Argon2id с уникальной солью в формате PHC:
// $argon2id$v=19$m=19456,t=2,p=1$<соль>$<хэш>.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// CheckPassword сравнивает пароль с хэшем за постоянное время. Параметры
// берутся из самого хэша, поэтому их можно усилить без миграции паролей.
func CheckPassword(hash, password string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false, errBadHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, errBadHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errBadHash
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want))) //nolint:gosec // len(want) ≤ длины хэша из БД
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// DummyPasswordHash — хэш случайного пароля. Проверка входа для
// несуществующего email сравнивает пароль с ним, чтобы время ответа не
// выдавало, зарегистрирован ли адрес (REQ-SEC-007).
var DummyPasswordHash = func() string {
	h, err := HashPassword(NewToken())
	if err != nil {
		panic(err)
	}
	return h
}()

// NewToken возвращает случайный токен из 256 бит в base64url (REQ-SEC-006).
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand не возвращает ошибок на поддерживаемых ОС
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken — SHA-256 токена для хранения в БД (ADR-0005, REQ-SEC-006).
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
