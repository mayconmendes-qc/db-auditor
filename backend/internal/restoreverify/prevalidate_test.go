package restoreverify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/osmendes/db-auditor/internal/config"
)

func TestImageMajor(t *testing.T) {
	for img, want := range map[string]int{"postgres:16": 16, "postgres:16.4-alpine": 16, "postgres:18": 18} {
		got, err := ImageMajor(img)
		if err != nil || got != want {
			t.Fatalf("%s: %d %v", img, got, err)
		}
	}
}

func TestFingerprintStableAndNoSecrets(t *testing.T) {
	a := FingerprintDSN("postgresql://user:secret@db.example.com:5432/app")
	b := FingerprintDSN("postgresql://other:other@db.example.com:5432/app")
	if a == "" || a != b {
		t.Fatalf("%q %q", a, b)
	}
}

func TestPrevalidateIsolationAndChecksum(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, "x.dump")
	_ = os.WriteFile(dump, []byte("PGDMP"), 0o600)
	sha, _, _ := FileSHA256(dump)
	if _, err := Prevalidate(PrevalidateInput{BackupPath: dump, Image: "postgres:16", ConfirmIsolated: false}); err == nil || !strings.Contains(err.Error(), "isolamento") {
		t.Fatalf("%v", err)
	}
	fp := FingerprintHostPortDB("127.0.0.1", "5432", "restore")
	if _, err := Prevalidate(PrevalidateInput{BackupPath: dump, Image: "postgres:16", ConfirmIsolated: true, Fingerprints: []string{fp}, CandidateFP: fp}); err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("%v", err)
	}
	if _, err := Prevalidate(PrevalidateInput{BackupPath: dump, Image: "postgres:14", SourceMajor: 16, ConfirmIsolated: true}); err == nil || !strings.Contains(err.Error(), "major") {
		t.Fatalf("%v", err)
	}
	if _, err := Prevalidate(PrevalidateInput{BackupPath: dump, Image: "postgres:16", ConfirmIsolated: true, ExpectedSHA: strings.Repeat("0", 64)}); err == nil || !strings.Contains(err.Error(), "diverge") {
		t.Fatalf("%v", err)
	}
	if got, err := Prevalidate(PrevalidateInput{BackupPath: dump, Image: "postgres:16", ConfirmIsolated: true, ExpectedSHA: sha}); err != nil || got.SHA256 != sha {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestCountDivergesTolerance(t *testing.T) {
	if countDiverges(1000, 1005) {
		t.Fatal("abs")
	}
	if !countDiverges(1_000_000, 1_010_000) {
		t.Fatal("pct")
	}
}

func TestSampleHashDisabledNotPartial(t *testing.T) {
	out := VerifySampleHashOptIn(t.Context(), nil, false, "", nil, nil)
	if out.Check.Status != StatusDisabled {
		t.Fatalf("%s", out.Check.Status)
	}
}

func TestRunRefusesSampleHashWithoutDSN(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, "x.dump")
	_ = os.WriteFile(dump, []byte("PGDMP"), 0o600)
	res := Run(t.Context(), Options{BackupPath: dump, EnvironmentID: "00000000-0000-4000-8000-000000000001", Image: "postgres:16", ConfirmIsolated: true, SampleHash: true, RuntimeExec: &FakeRuntime{}})
	if res.ExitCode != ExitUsage {
		t.Fatalf("%d %s", res.ExitCode, res.Evidence.Note)
	}
}

func TestRunRefusesWithoutIsolation(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, "x.dump")
	_ = os.WriteFile(dump, []byte("PGDMP"), 0o600)
	rt := &FakeRuntime{}
	res := Run(t.Context(), Options{BackupPath: dump, EnvironmentID: "00000000-0000-4000-8000-000000000001", Image: "postgres:16", ConfirmIsolated: false, RuntimeExec: rt})
	if res.ExitCode != ExitSecurity || len(rt.Calls) != 0 {
		t.Fatalf("%d calls=%d", res.ExitCode, len(rt.Calls))
	}
}

func TestRunRefusesMatchingFingerprint(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, "x.dump")
	_ = os.WriteFile(dump, []byte("PGDMP"), 0o600)
	fp := FingerprintHostPortDB("127.0.0.1", "5432", "restore")
	res := Run(t.Context(), Options{BackupPath: dump, EnvironmentID: "00000000-0000-4000-8000-000000000001", Image: "postgres:16", ConfirmIsolated: true, TargetFingerprints: []string{fp}, RuntimeExec: &FakeRuntime{}})
	if res.ExitCode != ExitSecurity {
		t.Fatalf("%d %s", res.ExitCode, res.Evidence.Note)
	}
}

func TestCorruptedChecksumExit1(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, "x.dump")
	_ = os.WriteFile(dump, []byte("PGDMP"), 0o600)
	res := Run(t.Context(), Options{BackupPath: dump, EnvironmentID: "00000000-0000-4000-8000-000000000001", Image: "postgres:16", ConfirmIsolated: true, ExpectedSHA256: strings.Repeat("ab", 32), RuntimeExec: &FakeRuntime{}})
	if res.ExitCode != ExitBlocked || res.Evidence.Succeeded {
		t.Fatalf("%d %v", res.ExitCode, res.Evidence.Succeeded)
	}
}

const testEnvID = "00000000-0000-4000-8000-000000000001"

func writeTestDump(t *testing.T) string {
	t.Helper()
	dump := filepath.Join(t.TempDir(), "x.dump")
	if err := os.WriteFile(dump, []byte("PGDMP"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dump
}

func ephemeralNamesFromCalls(calls [][]string) (name, volume string) {
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "volume" && c[1] == "create" {
			volume = c[len(c)-1]
		}
		if len(c) >= 2 && c[0] == "run" && c[1] == "-d" {
			for i := 0; i < len(c)-1; i++ {
				if c[i] == "--name" {
					name = c[i+1]
				}
			}
		}
	}
	return name, volume
}

func assertDestroyed(t *testing.T, rt *FakeRuntime, name, volume string) {
	t.Helper()
	if name == "" || volume == "" {
		t.Fatalf("missing ephemeral names name=%q volume=%q", name, volume)
	}
	var rm, volRm bool
	for _, c := range rt.Calls {
		if len(c) >= 3 && c[0] == "rm" && c[1] == "-f" && c[2] == name {
			rm = true
		}
		if len(c) >= 4 && c[0] == "volume" && c[1] == "rm" && c[2] == "-f" && c[3] == volume {
			volRm = true
		}
	}
	if !rm || !volRm {
		t.Fatalf("destroy incomplete rm=%v volRm=%v name=%s volume=%s calls=%v", rm, volRm, name, volume, rt.Calls)
	}
}

func checkByName(ev Evidence, name string) CheckResult {
	for _, c := range ev.Checks {
		if c.Name == name {
			return c
		}
	}
	return CheckResult{}
}

func assertNoSecretLeak(t *testing.T, res Result, secrets ...string) {
	t.Helper()
	blobs := []string{res.Evidence.Note, res.Summary, config.SanitizeError(res.Err)}
	for _, c := range res.Evidence.Checks {
		blobs = append(blobs, c.Detail)
	}
	for _, d := range res.Evidence.Divergences {
		blobs = append(blobs, d.Detail)
	}
	joined := strings.Join(blobs, "\n")
	for _, s := range secrets {
		if s != "" && strings.Contains(joined, s) {
			t.Fatalf("secret %q leaked in evidence/summary: %q", s, joined)
		}
	}
}

type closeOnlyConn struct{}

func (closeOnlyConn) Close(context.Context) error { return nil }
func (closeOnlyConn) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("unexpected QueryRow on closeOnlyConn")
}

func TestRestoreVerifyHappyPath(t *testing.T) {
	dump := writeTestDump(t)
	rt := &FakeRuntime{}
	res := Run(t.Context(), Options{
		BackupPath: dump, EnvironmentID: testEnvID, Image: "postgres:16",
		ConfirmIsolated: true, SampleHash: false, RuntimeExec: rt,
	})
	if res.ExitCode != ExitOK || !res.Evidence.Succeeded {
		t.Fatalf("exit=%d succeeded=%v note=%s", res.ExitCode, res.Evidence.Succeeded, res.Evidence.Note)
	}
	if got := checkByName(res.Evidence, "sample_hash"); got.Status != StatusDisabled {
		t.Fatalf("sample_hash status=%s want=%s", got.Status, StatusDisabled)
	}
	for _, name := range []string{"inventory", "counts", "amcheck"} {
		if got := checkByName(res.Evidence, name); got.Name == "" {
			t.Fatalf("missing check %s", name)
		}
	}
	name, volume := ephemeralNamesFromCalls(rt.Calls)
	assertDestroyed(t, rt, name, volume)
}

func TestMidRestoreFailureCleansContainerAndVolume(t *testing.T) {
	dump := writeTestDump(t)
	rt := &FakeRuntime{
		Match: func(args []string) (string, string, error, bool) {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "pg_restore --clean") {
				return "", "pg_restore: error: mid-restore failure", fmt.Errorf("exit 1"), true
			}
			return "", "", nil, false
		},
	}
	res := Run(t.Context(), Options{
		BackupPath: dump, EnvironmentID: testEnvID, Image: "postgres:16",
		ConfirmIsolated: true, RuntimeExec: rt,
	})
	if res.ExitCode != ExitBlocked || res.Evidence.Succeeded {
		t.Fatalf("exit=%d succeeded=%v note=%s", res.ExitCode, res.Evidence.Succeeded, res.Evidence.Note)
	}
	if !strings.Contains(res.Evidence.Note, "restauração falhou") {
		t.Fatalf("note=%s", res.Evidence.Note)
	}
	name, volume := ephemeralNamesFromCalls(rt.Calls)
	assertDestroyed(t, rt, name, volume)
}

func TestRunSampleHashWriteDSNRefused(t *testing.T) {
	prev := connectSource
	t.Cleanup(func() { connectSource = prev })
	secret := "WriteSecret-Pass-999"
	dsn := "postgresql://writer:" + secret + "@db.example.com:5432/app?sslmode=require"
	connectSource = func(context.Context, string) (sourceConn, error) {
		return nil, fmt.Errorf("target role has effective write or administration privilege")
	}
	dump := writeTestDump(t)
	rt := &FakeRuntime{}
	res := Run(t.Context(), Options{
		BackupPath: dump, EnvironmentID: testEnvID, Image: "postgres:16",
		ConfirmIsolated: true, SampleHash: true, SourceDSN: dsn, RuntimeExec: rt,
	})
	if res.ExitCode != ExitSecurity {
		t.Fatalf("exit=%d note=%s", res.ExitCode, res.Evidence.Note)
	}
	if got := checkByName(res.Evidence, "sample_hash"); got.Status != StatusFailed {
		t.Fatalf("sample_hash=%+v", got)
	}
	assertNoSecretLeak(t, res, secret, dsn)
	name, volume := ephemeralNamesFromCalls(rt.Calls)
	assertDestroyed(t, rt, name, volume)
}

func TestRunSampleHashReadOnlyPathNoDSNLeak(t *testing.T) {
	prev := connectSource
	t.Cleanup(func() { connectSource = prev })
	secret := "ReadOnly-Secret-XYZ"
	dsn := "postgresql://reader:" + secret + "@origin.example.com:5432/app"
	connectSource = func(context.Context, string) (sourceConn, error) {
		return closeOnlyConn{}, nil
	}
	dump := writeTestDump(t)
	rt := &FakeRuntime{}
	res := Run(t.Context(), Options{
		BackupPath: dump, EnvironmentID: testEnvID, Image: "postgres:16",
		ConfirmIsolated: true, SampleHash: true, SourceDSN: dsn, RuntimeExec: rt,
	})
	if res.ExitCode != ExitOK || !res.Evidence.Succeeded {
		t.Fatalf("exit=%d succeeded=%v note=%s", res.ExitCode, res.Evidence.Succeeded, res.Evidence.Note)
	}
	if got := checkByName(res.Evidence, "sample_hash"); got.Status != StatusSkipped {
		t.Fatalf("sample_hash=%+v (empty estimates → skipped after read-only connect)", got)
	}
	assertNoSecretLeak(t, res, secret, dsn)
	if leaked := config.SanitizeError(fmt.Errorf("boom %s", dsn)); strings.Contains(leaked, secret) {
		t.Fatalf("SanitizeError leaked secret: %s", leaked)
	}
	name, volume := ephemeralNamesFromCalls(rt.Calls)
	assertDestroyed(t, rt, name, volume)
}
