package loganomaly

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubmitLastLoginBodyOnlyRoleAndTime(t *testing.T) {
	var gotBody []byte
	var reqCount int
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"upserted":1}`))
	}))
	defer srv.Close()

	f := LastLoginFile{
		SchemaVersion: SchemaVersion,
		Entries: []LastLoginEntry{
			{Role: "app_readonly", LastLoginAt: "2026-10-19T14:30:00Z"},
		},
	}
	// Poison fields must not appear even if someone stuffed them into Role incorrectly —
	// body builder only copies role + last_login_at.
	client := &SubmitLastLoginHTTP{BaseURL: srv.URL, Token: "dba_test_token"}
	if err := SubmitLastLogin(context.Background(), client, "11111111-1111-1111-1111-111111111111", f); err != nil {
		t.Fatal(err)
	}
	if reqCount != 1 {
		t.Fatalf("expected exactly 1 request, got %d", reqCount)
	}
	if !strings.HasSuffix(gotPath, "/account-last-logins") {
		t.Fatalf("path=%s", gotPath)
	}
	if err := ValidateLastLoginOnlyRoleAndTime(gotBody); err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(gotBody))
	for _, banned := range []string{"1.2.3.4", "@", "application", "email", "template"} {
		if strings.Contains(lower, banned) {
			t.Fatalf("banned token %q present in body", banned)
		}
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &probe); err != nil {
		t.Fatal(err)
	}
	for k := range probe {
		if k != "schema_version" && k != "entries" {
			t.Fatalf("extra top key %s", k)
		}
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(probe["entries"], &entries); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		for k := range e {
			if k != "role" && k != "last_login_at" {
				t.Fatalf("extra entry key %s", k)
			}
		}
	}
}

func TestSubmitLastLoginOtherEnvironment403ExitMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()
	client := &SubmitLastLoginHTTP{BaseURL: srv.URL, Token: "dba_other"}
	err := SubmitLastLogin(context.Background(), client, "22222222-2222-2222-2222-222222222222", LastLoginFile{
		SchemaVersion: SchemaVersion,
		Entries:       []LastLoginEntry{{Role: "r", LastLoginAt: "2026-10-19T00:00:00Z"}},
	})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildLastLoginSubmitBodyClosed(t *testing.T) {
	f := LastLoginFile{
		SchemaVersion: "1",
		Entries:       []LastLoginEntry{{Role: " a ", LastLoginAt: "2026-01-01T00:00:00Z"}},
	}
	body := BuildLastLoginSubmitBody(f)
	raw, _ := json.Marshal(body)
	if err := ValidateLastLoginOnlyRoleAndTime(raw); err != nil {
		t.Fatal(err)
	}
	if body.Entries[0].Role != "a" {
		t.Fatalf("role=%q", body.Entries[0].Role)
	}
}
