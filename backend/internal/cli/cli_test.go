package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpListsCommands(t *testing.T) {
	var buf bytes.Buffer
	WriteHelp(&buf)
	out := buf.String()
	for _, cmd := range []string{"migration-risk", "recover-operator", "help"} {
		if !strings.Contains(out, cmd) {
			t.Fatalf("help missing %q:\n%s", cmd, out)
		}
	}
	for _, code := range []string{"0", "1", "2", "3", "4"} {
		if !strings.Contains(out, code) {
			t.Fatalf("help missing exit code %s", code)
		}
	}
}

func TestRefuseSecretFlags(t *testing.T) {
	cases := [][]string{
		{"--token", "secret-value"},
		{"--token=secret-value"},
		{"--password=x"},
		{"--dsn=postgres://u:p@h/db"},
	}
	for _, args := range cases {
		err := RefuseSecretFlags(args)
		if err == nil {
			t.Fatalf("expected refusal for %v", args)
		}
		msg := err.Error()
		if strings.Contains(msg, "secret-value") || strings.Contains(msg, "postgres://") {
			t.Fatalf("error leaked secret value: %s", msg)
		}
	}
	if err := RefuseSecretFlags([]string{"--path", "./migrations", "--format", "json"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestExitCodeTable(t *testing.T) {
	want := map[int]string{
		ExitOK:              "ok",
		ExitBlocked:         "blocked",
		ExitUsage:           "usage",
		ExitInsufficient:    "insufficient",
		ExitSecurityRefused: "security_refused",
	}
	for code, name := range want {
		if got := ExitCodeName(code); got != name {
			t.Errorf("ExitCodeName(%d)=%q want %q", code, got, name)
		}
	}
}

func TestFormatFlag(t *testing.T) {
	for raw, want := range map[string]string{"": "md", "md": "md", "markdown": "md", "JSON": "json"} {
		got, err := FormatFlag(raw)
		if err != nil || got != want {
			t.Fatalf("FormatFlag(%q)=%q,%v want %q", raw, got, err, want)
		}
	}
	if _, err := FormatFlag("sarif"); err == nil {
		t.Fatal("expected invalid format")
	}
}

func TestRequireCLITokenFromEnv(t *testing.T) {
	t.Setenv("AUDITOR_CLI_TOKEN", "")
	t.Setenv("AUDITOR_CLI_TOKEN_FILE", "")
	if _, err := RequireCLIToken(); err == nil {
		t.Fatal("expected missing token error")
	}
	t.Setenv("AUDITOR_CLI_TOKEN", "dba_test_token")
	got, err := RequireCLIToken()
	if err != nil || got != "dba_test_token" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestIsHelpArg(t *testing.T) {
	if !IsHelpArg(nil) || !IsHelpArg([]string{"help"}) || !IsHelpArg([]string{"--help"}) {
		t.Fatal("expected help")
	}
	if IsHelpArg([]string{"migration-risk"}) {
		t.Fatal("migration-risk is not help")
	}
}
