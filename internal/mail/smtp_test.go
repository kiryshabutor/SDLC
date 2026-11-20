package mail

import (
	"bufio"
	"context"
	"errors"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/kiryshabutor/SDLC/internal/domain"
)

// fakeSMTP — минимальный SMTP-сервер без TLS: принимает одно соединение за
// раз, отклоняет получателей reject@… кодом 550 и складывает письма в канал.
func fakeSMTP(t *testing.T) (port int, got chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	got = make(chan string, 4)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(conn, got)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func serve(conn net.Conn, got chan<- string) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	say := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
	say("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			say("250-fake")
			say("250 8BITMIME")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			if strings.Contains(cmd, "REJECT@") {
				say("550 no such user")
			} else {
				say("250 ok")
			}
		case cmd == "DATA":
			say("354 go ahead")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil || l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			got <- b.String()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("502 unknown")
		}
	}
}

func TestSMTPSend(t *testing.T) {
	port, got := fakeSMTP(t)
	s := &SMTP{Cfg: Config{Host: "127.0.0.1", Port: port, From: "Охотник за скидками <noreply@deals.example>", TLS: TLSNone, Timeout: 5 * time.Second}}
	msg := domain.Message{Channel: domain.ChannelEmail, Kind: domain.KindVerify, Recipient: "kirill@example.com",
		Subject: "Подтвердите email", Body: "Здравствуйте!\nСсылка: https://deals.example/verify?token=abc\n"}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	raw := <-got
	parsed, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	subj, _ := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if subj != "Подтвердите email" || parsed.Header.Get("To") != "kirill@example.com" ||
		!strings.Contains(parsed.Header.Get("Content-Type"), "utf-8") || parsed.Header.Get("Message-Id") == "" {
		t.Fatalf("headers: %v (subject %q)", parsed.Header, subj)
	}
	b, err := io.ReadAll(quotedprintable.NewReader(parsed.Body))
	if err != nil || !strings.Contains(string(b), "Здравствуйте!\r\nСсылка: https://deals.example/verify?token=abc") {
		t.Fatalf("body = %q", b)
	}
}

func TestSMTPErrors(t *testing.T) {
	port, _ := fakeSMTP(t)
	s := &SMTP{Cfg: Config{Host: "127.0.0.1", Port: port, From: "noreply@deals.example", TLS: TLSNone, Timeout: 5 * time.Second}}
	err := s.Send(context.Background(), domain.Message{Recipient: "reject@example.com", Body: "x"})
	if !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("550: %v", err)
	}
	if err := s.Send(context.Background(), domain.Message{Recipient: "not an address", Body: "x"}); !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("bad address: %v", err)
	}
	// STARTTLS обязателен: сервер без него не принимает письмо.
	s.Cfg.TLS = TLSStartTLS
	if err := s.Send(context.Background(), domain.Message{Recipient: "a@example.com", Body: "x"}); err == nil || errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("без STARTTLS: %v", err)
	}
	// Недоступный сервер — временная ошибка.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	s.Cfg.Port, s.Cfg.TLS = closed, TLSNone
	if err := s.Send(context.Background(), domain.Message{Recipient: "a@example.com", Body: "x"}); err == nil || errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("недоступен: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	ok := Config{Host: "smtp.example.com", Port: 587, From: "noreply@example.com", TLS: TLSStartTLS}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Config{
		{Port: 587, From: "a@b.c", TLS: TLSStartTLS},
		{Host: "h", Port: 587, From: "bad", TLS: TLSStartTLS},
		{Host: "h", Port: 587, From: "a@b.c", TLS: "ssl"},
		{Host: "h", From: "a@b.c", TLS: TLSNone},
	} {
		if c.Validate() == nil {
			t.Errorf("%+v: ожидалась ошибка", c)
		}
	}
}
