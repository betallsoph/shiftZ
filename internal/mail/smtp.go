package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// Sender delivers plain-text email.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SMTPConfig configures an SMTP sender.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Enabled reports whether SMTP is configured enough to send mail.
func (c SMTPConfig) Enabled() bool {
	return strings.TrimSpace(c.Host) != "" && strings.TrimSpace(c.From) != ""
}

// NewSMTP returns an SMTP-backed Sender.
func NewSMTP(cfg SMTPConfig) Sender {
	return &smtpSender{cfg: cfg}
}

type smtpSender struct {
	cfg SMTPConfig
}

func (s *smtpSender) Send(ctx context.Context, to, subject, body string) error {
	_ = ctx
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("mail: empty recipient")
	}
	port := s.cfg.Port
	if port == 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, port)
	msg := strings.Join([]string{
		"From: " + s.cfg.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}
	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}
