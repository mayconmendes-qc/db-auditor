package loganomaly

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Profile extracts time, level and message from a JSON object.
type Profile interface {
	Name() string
	Parse(raw map[string]json.RawMessage) (ts time.Time, level, message string, err error)
	// RoleFromJSON returns the role name for last-login (postgres only); empty otherwise.
	RoleFromJSON(raw map[string]json.RawMessage) string
}

// NewProfile builds a field profile.
func NewProfile(name, timeField, levelField, messageField string) (Profile, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case ProfilePostgresJSONLog, "postgres", "pg":
		return postgresProfile{}, nil
	case ProfileGenerico, "generic", "genérico":
		tf := strings.TrimSpace(timeField)
		lf := strings.TrimSpace(levelField)
		mf := strings.TrimSpace(messageField)
		if tf == "" {
			return nil, fmt.Errorf("perfil generico exige --time-field")
		}
		if mf == "" {
			return nil, fmt.Errorf("perfil generico exige --message-field")
		}
		if lf == "" {
			lf = "level"
		}
		return genericProfile{timeField: tf, levelField: lf, messageField: mf}, nil
	default:
		return nil, fmt.Errorf("perfil desconhecido %q (use postgres-jsonlog ou generico)", name)
	}
}

func jsonString(raw map[string]json.RawMessage, key string) (string, bool) {
	v, ok := raw[key]
	if !ok || len(v) == 0 || string(v) == "null" {
		return "", false
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		// number or other — stringify without quotes when possible
		s = strings.Trim(string(v), `"`)
		return s, s != ""
	}
	return s, true
}

func normalizeLevel(s string) (bucket, raw string) {
	raw = strings.TrimSpace(s)
	u := strings.ToUpper(raw)
	switch u {
	case "ERROR", "FATAL", "PANIC", "CRITICAL", "ALERT", "EMERG", "EMERGENCY":
		return LevelError, raw
	case "WARNING", "WARN":
		return LevelWarning, raw
	case "LOG", "INFO", "NOTICE", "DEBUG", "DEBUG1", "DEBUG2", "DEBUG3", "DEBUG4", "DEBUG5", "TRACE":
		return LevelInfo, raw
	default:
		if strings.Contains(u, "ERR") || strings.Contains(u, "FATAL") || strings.Contains(u, "PANIC") {
			return LevelError, raw
		}
		if strings.Contains(u, "WARN") {
			return LevelWarning, raw
		}
		return LevelInfo, raw
	}
}

// parseFlexibleTime accepts PG jsonlog timestamps and RFC3339.
func parseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("horário vazio")
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.000 MST",
		"2006-01-02 15:04:05.000 -07",
		"2006-01-02 15:04:05.000 -0700",
		"2006-01-02 15:04:05.999999 MST",
		"2006-01-02 15:04:05.999999 -07",
		"2006-01-02 15:04:05.999999 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -07",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
	}
	var firstErr error
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.UTC(), nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	// Strip trailing timezone name when numeric offset also present is rare;
	// try cutting last token if it looks like a zone abbreviation.
	if i := strings.LastIndex(s, " "); i > 0 {
		if t, err := time.Parse("2006-01-02 15:04:05.000", s[:i]); err == nil {
			return t.UTC(), nil
		}
		if t, err := time.Parse("2006-01-02 15:04:05", s[:i]); err == nil {
			return t.UTC(), nil
		}
	}
	if firstErr != nil {
		return time.Time{}, fmt.Errorf("horário inválido")
	}
	return time.Time{}, fmt.Errorf("horário inválido")
}
