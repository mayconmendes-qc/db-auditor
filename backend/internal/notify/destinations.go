package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// Destination is one per-environment delivery target (full URL/e-mail kept in memory only).
type Destination struct {
	ID      string
	Channel string // email | webhook
	Target  string
	Format  string // generic (slack rejected upstream until plan 04)
}

// DigestPayload is the webhook summary for Kind=query_digest (counts + link only).
type DigestPayload struct {
	Event
	Link             string `json:"link,omitempty"`
	RegressionCount  int    `json:"regression_count"`
	NewCount         int    `json:"new_count"`
	GoneCount        int    `json:"gone_count"`
	ImprovementCount int    `json:"improvement_count"`
	State            string `json:"state,omitempty"`
}

// SendToDestinations delivers to env destinations. Returns per-destination errors
// (sanitized by the caller). Empty destinations means the caller should fall back
// to Config.Send.
func SendToDestinations(ctx context.Context, dests []Destination, event Event, body any) []error {
	var errs []error
	for _, d := range dests {
		var err error
		switch d.Channel {
		case "webhook":
			err = postDestination(ctx, d.Target, body)
		case "email":
			err = mailDestination(d.Target, event)
		default:
			err = fmt.Errorf("canal desconhecido")
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.ID, err))
		} else {
			errs = append(errs, nil)
		}
	}
	return errs
}

func postDestination(ctx context.Context, webhookURL string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
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

func mailDestination(to string, event Event) error {
	cfg := FromEnv()
	if cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP não configurado")
	}
	port := cfg.SMTPPort
	if port == "" {
		port = "25"
	}
	from := cfg.From
	if from == "" {
		from = "db-auditor@localhost"
	}
	msg := []byte("Subject: DB Auditor " + event.Kind + "\r\n\r\n" + event.Title + "\r\n")
	addr := cfg.SMTPHost + ":" + port
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}

// GlobalFallbackChannels describes which global channels would be used.
func (c Config) GlobalFallbackChannels() []string {
	var out []string
	if strings.TrimSpace(c.WebhookURL) != "" {
		out = append(out, "global_webhook")
	}
	if strings.TrimSpace(c.EmailTo) != "" && strings.TrimSpace(c.SMTPHost) != "" {
		out = append(out, "global_email")
	}
	return out
}
