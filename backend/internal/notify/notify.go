// Package notify sends audit alerts without failing the audit run.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// Event is a sanitized alert. It must not carry a DSN, password, or raw SQL.
type Event struct {
	Kind          string `json:"kind"`
	EnvironmentID string `json:"environment_id"`
	DedupKey      string `json:"dedup_key"`
	Title         string `json:"title"`
	Severity      string `json:"severity,omitempty"`
}

// Config is empty when alerting is off.
type Config struct {
	WebhookURL string
	EmailTo    string
	SMTPHost   string
	SMTPPort   string
	SMTPUser   string
	SMTPPass   string
	From       string
}

func FromEnv() Config {
	return Config{
		WebhookURL: strings.TrimSpace(os.Getenv("AUDITOR_ALERT_WEBHOOK_URL")),
		EmailTo:    strings.TrimSpace(os.Getenv("AUDITOR_ALERT_EMAIL_TO")),
		SMTPHost:   strings.TrimSpace(os.Getenv("AUDITOR_ALERT_SMTP_HOST")),
		SMTPPort:   strings.TrimSpace(os.Getenv("AUDITOR_ALERT_SMTP_PORT")),
		SMTPUser:   os.Getenv("AUDITOR_ALERT_SMTP_USER"),
		SMTPPass:   os.Getenv("AUDITOR_ALERT_SMTP_PASSWORD"),
		From:       strings.TrimSpace(os.Getenv("AUDITOR_ALERT_EMAIL_FROM")),
	}
}

func (c Config) Enabled() bool {
	return c.WebhookURL != "" || (c.EmailTo != "" && c.SMTPHost != "")
}

// Send delivers one event. A transport error is returned to the caller, which must ignore it.
func (c Config) Send(ctx context.Context, event Event) error {
	if !c.Enabled() {
		return nil
	}
	var errs []string
	if c.WebhookURL != "" {
		if err := c.post(ctx, event); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if c.EmailTo != "" && c.SMTPHost != "" {
		if err := c.mail(event); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (c Config) post(ctx context.Context, event Event) error {
	body, _ := json.Marshal(event)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", res.StatusCode)
	}
	return nil
}

func (c Config) mail(event Event) error {
	port := c.SMTPPort
	if port == "" {
		port = "25"
	}
	from := c.From
	if from == "" {
		from = "db-auditor@localhost"
	}
	msg := []byte("Subject: DB Auditor " + event.Kind + "\r\n\r\n" + event.Title + "\r\n")
	addr := c.SMTPHost + ":" + port
	var auth smtp.Auth
	if c.SMTPUser != "" {
		auth = smtp.PlainAuth("", c.SMTPUser, c.SMTPPass, c.SMTPHost)
	}
	return smtp.SendMail(addr, auth, from, []string{c.EmailTo}, msg)
}
