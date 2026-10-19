package loganomaly

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// LastLoginEntry is the closed struct for RF-08 / ADR-0001 D10.
// Only role and last_login_at — never IP, database, or application.
type LastLoginEntry struct {
	Role        string `json:"role"`
	LastLoginAt string `json:"last_login_at"` // RFC3339 UTC
}

// LastLoginFile is the local file format that plan 07 L6 / plan 10 L5 will accept.
type LastLoginFile struct {
	SchemaVersion string           `json:"schema_version"`
	Entries       []LastLoginEntry `json:"entries"`
}

// LastLoginAgg tracks the latest login time per role (in-memory only).
type LastLoginAgg struct {
	byRole map[string]time.Time
}

// NewLastLoginAgg creates an empty aggregator.
func NewLastLoginAgg() *LastLoginAgg {
	return &LastLoginAgg{byRole: make(map[string]time.Time)}
}

// Observe records a connection-authorized event's role and time.
// role must already be extracted; IP/database must not be passed.
func (a *LastLoginAgg) Observe(role string, ts time.Time) {
	role = trimRole(role)
	if role == "" || ts.IsZero() {
		return
	}
	prev, ok := a.byRole[role]
	if !ok || ts.After(prev) {
		a.byRole[role] = ts.UTC()
	}
}

func trimRole(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// File builds the schema_versioned payload sorted by role (deterministic).
func (a *LastLoginAgg) File() LastLoginFile {
	roles := make([]string, 0, len(a.byRole))
	for r := range a.byRole {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	entries := make([]LastLoginEntry, 0, len(roles))
	for _, r := range roles {
		entries = append(entries, LastLoginEntry{
			Role:        r,
			LastLoginAt: a.byRole[r].UTC().Format(time.RFC3339),
		})
	}
	return LastLoginFile{SchemaVersion: SchemaVersion, Entries: entries}
}

// WriteLastLoginJSON writes the aggregate to path (or stdout when path is "-").
func WriteLastLoginJSON(path string, f LastLoginFile) error {
	var w io.Writer
	if path == "" || path == "-" {
		w = os.Stdout
	} else {
		file, err := os.Create(path) //nolint:gosec // G304: operator-chosen --last-login-out
		if err != nil {
			return fmt.Errorf("não foi possível escrever último login")
		}
		defer file.Close()
		w = file
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(f)
}

// ValidateLastLoginOnlyRoleAndTime fails if any entry JSON would carry extra fields
// when round-tripped through the closed struct (test helper).
func ValidateLastLoginOnlyRoleAndTime(raw []byte) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return err
	}
	for k := range probe {
		if k != "schema_version" && k != "entries" {
			return fmt.Errorf("campo extra no arquivo: %s", k)
		}
	}
	var f LastLoginFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	for _, e := range f.Entries {
		b, _ := json.Marshal(e)
		var m map[string]json.RawMessage
		_ = json.Unmarshal(b, &m)
		for k := range m {
			if k != "role" && k != "last_login_at" {
				return fmt.Errorf("campo extra na entrada: %s", k)
			}
		}
	}
	return nil
}
