package restoreverify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
