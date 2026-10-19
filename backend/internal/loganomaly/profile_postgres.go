package loganomaly

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// postgresProfile maps PG 15+ jsonlog fields (ADR-0001 D10).
type postgresProfile struct{}

func (postgresProfile) Name() string { return ProfilePostgresJSONLog }

func (postgresProfile) Parse(raw map[string]json.RawMessage) (time.Time, string, string, error) {
	tsStr, ok := jsonString(raw, "timestamp")
	if !ok || tsStr == "" {
		return time.Time{}, "", "", fmt.Errorf("campo timestamp ausente")
	}
	ts, err := parseFlexibleTime(tsStr)
	if err != nil {
		return time.Time{}, "", "", err
	}
	sev, _ := jsonString(raw, "error_severity")
	level, _ := normalizeLevel(sev)
	msg, ok := jsonString(raw, "message")
	if !ok {
		msg = ""
	}
	return ts, level, msg, nil
}

func (postgresProfile) RoleFromJSON(raw map[string]json.RawMessage) string {
	u, ok := jsonString(raw, "user")
	if !ok {
		return ""
	}
	return strings.TrimSpace(u)
}

// IsConnectionAuthorized reports postgres log_connections success messages.
func IsConnectionAuthorized(message string) bool {
	m := strings.ToLower(message)
	return strings.Contains(m, "connection authorized")
}

// RoleFromConnMessage extracts user= from a connection authorized message.
// Prefer RoleFromJSON when the jsonlog user field is present.
func RoleFromConnMessage(message string) string {
	const key = "user="
	lower := strings.ToLower(message)
	i := strings.Index(lower, key)
	if i < 0 {
		return ""
	}
	rest := message[i+len(key):]
	end := len(rest)
	for j, r := range rest {
		if r == ' ' || r == ',' || r == '\t' {
			end = j
			break
		}
	}
	return strings.TrimSpace(rest[:end])
}
