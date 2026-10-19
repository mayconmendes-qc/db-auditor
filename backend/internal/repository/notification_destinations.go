package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

// NotificationDestination is a per-environment alert/digest destination.
// Webhook Target is a secret: API responses use TargetMasked only.
type NotificationDestination struct {
	ID            string    `json:"id"`
	EnvironmentID string    `json:"environment_id"`
	Channel       string    `json:"channel"` // email | webhook
	TargetMasked  string    `json:"target_masked"`
	Format        string    `json:"format"` // generic | slack
	Active        bool      `json:"active"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NotificationDestinationInput is the operator write model (full target).
type NotificationDestinationInput struct {
	Channel string `json:"channel"`
	Target  string `json:"target"`
	Format  string `json:"format"`
	Active  bool   `json:"active"`
}

// MaskDestination hides webhook secrets and keeps e-mail readable.
func MaskDestination(channel, target string) string {
	target = strings.TrimSpace(target)
	if channel == "email" {
		return target
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return "(webhook)"
	}
	return u.Scheme + "://" + u.Host + "/***"
}

// ValidateNotificationDestination checks channel, format and SSRF-safe webhook URL.
func ValidateNotificationDestination(in NotificationDestinationInput) error {
	in.Channel = strings.TrimSpace(strings.ToLower(in.Channel))
	in.Format = strings.TrimSpace(strings.ToLower(in.Format))
	in.Target = strings.TrimSpace(in.Target)
	if in.Format == "" {
		in.Format = "generic"
	}
	switch in.Channel {
	case "email":
		if len(in.Target) == 0 || len(in.Target) > 320 {
			return fmt.Errorf("e-mail inválido")
		}
		addr, err := mail.ParseAddress(in.Target)
		if err != nil || addr.Address == "" || strings.ContainsAny(addr.Address, " \t\r\n") {
			return fmt.Errorf("e-mail inválido")
		}
	case "webhook":
		if in.Format == "slack" {
			return fmt.Errorf("formato slack ainda não disponível")
		}
		if in.Format != "generic" {
			return fmt.Errorf("formato de webhook inválido")
		}
		if err := validateWebhookURL(in.Target); err != nil {
			return err
		}
	default:
		return fmt.Errorf("canal inválido")
	}
	return nil
}

func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("webhook exige URL https válida")
	}
	if u.User != nil {
		return fmt.Errorf("webhook não pode embutir credenciais")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook exige URL https válida")
	}
	if isBlockedWebhookHost(host) {
		return fmt.Errorf("destino de webhook recusado")
	}
	return nil
}

func isBlockedWebhookHost(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || h == "metadata.google.internal" {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		// Hostname: still block obvious local names; DNS rebinding is mitigated
		// by operator allowlists documented in ops (URL stored, delivery uses https client).
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	return false
}

// ListNotificationDestinations returns destinations with masked targets.
func (s *Store) ListNotificationDestinations(ctx context.Context, env string) ([]NotificationDestination, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,environment_id::text,channel,target,format,active,created_by,created_at,updated_at
FROM notification_destination WHERE environment_id=$1::uuid ORDER BY created_at ASC,id ASC`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NotificationDestination{}
	for rows.Next() {
		var d NotificationDestination
		var target string
		if err := rows.Scan(&d.ID, &d.EnvironmentID, &d.Channel, &target, &d.Format, &d.Active, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.TargetMasked = MaskDestination(d.Channel, target)
		out = append(out, d)
	}
	return out, rows.Err()
}

// ReplaceNotificationDestinations replaces all destinations for an environment.
func (s *Store) ReplaceNotificationDestinations(ctx context.Context, env, actor string, inputs []NotificationDestinationInput) ([]NotificationDestination, error) {
	if len(inputs) > 50 {
		return nil, fmt.Errorf("limite de 50 destinatários")
	}
	normalized := make([]NotificationDestinationInput, 0, len(inputs))
	for _, in := range inputs {
		in.Channel = strings.TrimSpace(strings.ToLower(in.Channel))
		in.Format = strings.TrimSpace(strings.ToLower(in.Format))
		if in.Format == "" {
			in.Format = "generic"
		}
		in.Target = strings.TrimSpace(in.Target)
		if err := ValidateNotificationDestination(in); err != nil {
			return nil, err
		}
		normalized = append(normalized, in)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `DELETE FROM notification_destination WHERE environment_id=$1::uuid`, env); err != nil {
		return nil, err
	}
	for _, in := range normalized {
		if _, err = tx.Exec(ctx, `INSERT INTO notification_destination(environment_id,channel,target,format,active,created_by)
VALUES($1::uuid,$2,$3,$4,$5,$6)`, env, in.Channel, in.Target, in.Format, in.Active, actor); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListNotificationDestinations(ctx, env)
}

// listActiveDestinationSecrets returns full targets for delivery (never expose via API).
func (s *Store) listActiveDestinationSecrets(ctx context.Context, env string) ([]struct {
	ID, Channel, Target, Format string
}, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,channel,target,format FROM notification_destination
WHERE environment_id=$1::uuid AND active ORDER BY created_at ASC,id ASC`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID, Channel, Target, Format string
	}
	for rows.Next() {
		var d struct {
			ID, Channel, Target, Format string
		}
		if err := rows.Scan(&d.ID, &d.Channel, &d.Target, &d.Format); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// RecordQueryDigestDelivery inserts a delivery row.
func (s *Store) RecordQueryDigestDelivery(ctx context.Context, digestID, destinationID, channel, masked, status string, attempt int, errSanitized string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO query_digest_delivery(digest_id,destination_id,channel,target_masked,status,attempt,error_sanitized)
VALUES($1::uuid,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7)`, digestID, destinationID, channel, masked, status, attempt, errSanitized)
	return err
}

// DigestAlreadyDelivered reports whether ClaimAlert already recorded success for this digest.
func (s *Store) DigestAlreadyDelivered(ctx context.Context, env, digestID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM alert_delivery WHERE environment_id=$1::uuid AND dedup_key=$2)`, env, digestID).Scan(&exists)
	return exists, err
}

// ErrNotificationScope is returned when the environment does not exist.
var ErrNotificationScope = errors.New("notification scope does not belong to environment")

func (s *Store) ensureEnvironment(ctx context.Context, env string) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_environment WHERE id=$1::uuid)`, env).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotificationScope
	}
	return nil
}
