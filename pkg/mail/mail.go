package mail

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

// Mailer holds SMTP configuration.
type Mailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// NewMailerFromEnv builds a Mailer reading settings from environment variables.
// It uses the following env vars: SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS
// Optionally set SMTP_FROM to override From address (defaults to SMTP_USER).
func NewMailerFromEnv() *Mailer {
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")

	port := 587
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}
	if from == "" {
		from = user
	}

	return &Mailer{Host: host, Port: port, Username: user, Password: pass, From: from}
}

// Send sends an email with the provided subject and HTML body to the recipients.
// The body can be plain text or HTML; the Content-Type header will be set to HTML.
func (m *Mailer) Send(to []string, subject, body string) error {
	if m.Host == "" || m.Port == 0 {
		log.Printf("mailer config: host=%s, port=%d, user=%s", m.Host, m.Port, m.Username)
		return fmt.Errorf("smtp host/port not configured")
	}
	if len(to) == 0 {
		log.Printf("no recipients provided for email with subject: %s", subject)
		return fmt.Errorf("no recipients provided")
	}
	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)

	// build headers
	header := make(map[string]string)
	header["From"] = m.From
	header["To"] = strings.Join(to, ",")
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"UTF-8\""

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	log.Printf("sending email to %s with subject: %s", strings.Join(to, ","), subject)

	// Support SMTPS on port 465 (implicit TLS) and STARTTLS on other ports (eg. 587)
	if m.Port == 465 {
		tlsConfig := &tls.Config{ServerName: m.Host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls dial: %w", err)
		}
		c, err := smtp.NewClient(conn, m.Host)
		if err != nil {
			return fmt.Errorf("new smtp client: %w", err)
		}
		defer c.Quit()

		if m.Username != "" || m.Password != "" {
			auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("auth failed: %w", err)
			}
		}

		if err := c.Mail(m.From); err != nil {
			return fmt.Errorf("mail from: %w", err)
		}
		for _, rcpt := range to {
			if err := c.Rcpt(rcpt); err != nil {
				return fmt.Errorf("rcpt to %s: %w", rcpt, err)
			}
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("data start: %w", err)
		}
		if _, err := w.Write([]byte(msg.String())); err != nil {
			_ = w.Close()
			return fmt.Errorf("write data: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("close data: %w", err)
		}
		return c.Quit()
	}

	// Fallback: plain connection and STARTTLS when available
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: m.Host}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	if m.Username != "" || m.Password != "" {
		auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}
	}

	if err := c.Mail(m.From); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt to %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data start: %w", err)
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		_ = w.Close()
		return fmt.Errorf("write data: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return c.Quit()
}

// SendSimple is a convenience wrapper for sending to a single recipient.
func (m *Mailer) SendSimple(to, subject, body string) error {
	return m.Send([]string{to}, subject, body)
}
