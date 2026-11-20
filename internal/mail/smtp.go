// Package mail — отправка писем по SMTP (REQ-INT-010, DV-013).
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// Режимы шифрования соединения с SMTP-сервером.
const (
	TLSStartTLS = "starttls" // порт 587: обычное соединение, затем STARTTLS (обязательно)
	TLSImplicit = "tls"      // порт 465: SMTPS, TLS с первого байта
	TLSNone     = "none"     // только для тестового Mailpit в той же сети Docker
)

// Config — параметры SMTP из окружения (SMTP_HOST, SMTP_PORT, SMTP_USER,
// SMTP_PASSWORD, SMTP_FROM, SMTP_TLS).
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	TLS      string
	Timeout  time.Duration
}

// Validate проверяет конфигурацию при запуске.
func (c Config) Validate() error {
	if c.Host == "" || c.Port <= 0 {
		return errors.New("SMTP_HOST and SMTP_PORT are required")
	}
	if _, err := mail.ParseAddress(c.From); err != nil {
		return fmt.Errorf("SMTP_FROM: %w", err)
	}
	switch c.TLS {
	case TLSStartTLS, TLSImplicit, TLSNone:
	default:
		return fmt.Errorf("SMTP_TLS must be starttls, tls or none, got %q", c.TLS)
	}
	return nil
}

// SMTP — Notifier канала email.
type SMTP struct {
	Cfg Config
	Now func() time.Time
}

// Channel возвращает domain.ChannelEmail.
func (s *SMTP) Channel() string { return domain.ChannelEmail }

// Send отправляет одно письмо. Ответ сервера 5xx на адрес получателя или
// данные письма считается окончательной ошибкой (domain.ErrPermanent),
// остальные ошибки — временными (повтор через 5 минут).
func (s *SMTP) Send(ctx context.Context, msg domain.Message) error {
	if _, err := mail.ParseAddress(msg.Recipient); err != nil {
		return fmt.Errorf("%w: bad recipient: %w", domain.ErrPermanent, err)
	}
	data, err := s.build(msg)
	if err != nil {
		return err
	}
	timeout := s.Cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	addr := net.JoinHostPort(s.Cfg.Host, strconv.Itoa(s.Cfg.Port))
	tlsCfg := &tls.Config{ServerName: s.Cfg.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	if s.Cfg.TLS == TLSImplicit {
		conn, err = (&tls.Dialer{Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	c, err := smtp.NewClient(conn, s.Cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()

	if s.Cfg.TLS == TLSStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("smtp: server does not support STARTTLS")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if s.Cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.Cfg.User, s.Cfg.Password, s.Cfg.Host)); err != nil {
			return err
		}
	}
	from, _ := mail.ParseAddress(s.Cfg.From)
	if err := c.Mail(from.Address); err != nil {
		return classify(err)
	}
	if err := c.Rcpt(msg.Recipient); err != nil {
		return classify(err)
	}
	w, err := c.Data()
	if err != nil {
		return classify(err)
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return classify(err)
	}
	return c.Quit()
}

func classify(err error) error {
	var te *textproto.Error
	if errors.As(err, &te) && te.Code >= 500 {
		return fmt.Errorf("%w: %w", domain.ErrPermanent, err)
	}
	return err
}

// build собирает письмо text/plain в UTF-8 с quoted-printable телом и
// заголовком Subject в кодировке RFC 2047.
func (s *SMTP) build(msg domain.Message) ([]byte, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	from, err := mail.ParseAddress(s.Cfg.From)
	if err != nil {
		return nil, err
	}
	domainPart := from.Address[strings.LastIndexByte(from.Address, '@')+1:]
	id := make([]byte, 12)
	_, _ = rand.Read(id)

	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", from.String())
	header("To", msg.Recipient)
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domainPart+">")
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	body := strings.ReplaceAll(strings.ReplaceAll(msg.Body, "\r\n", "\n"), "\n", "\r\n")
	if _, err := qp.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
