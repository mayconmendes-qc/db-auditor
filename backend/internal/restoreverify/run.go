package restoreverify

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ExitOK           = 0
	ExitBlocked      = 1
	ExitUsage        = 2
	ExitInsufficient = 3
	ExitSecurity     = 4
)

func Run(ctx context.Context, opt Options) Result {
	performed := time.Now().UTC()
	ev := Evidence{
		Kind: KindTargetRestoreDrill, EnvironmentID: opt.EnvironmentID, CLIVersion: opt.CLIVersion,
		PerformedAt: performed, Checks: []CheckResult{}, Divergences: []Divergence{},
	}
	fail := func(code int, note string) Result {
		ev.Succeeded = false
		ev.Note = note
		return Result{ExitCode: code, Evidence: ev, Summary: RenderSummary(ev), Err: fmt.Errorf("%s", note)}
	}
	if opt.Timeout <= 0 {
		opt.Timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()
	if opt.SampleHash && strings.TrimSpace(opt.SourceDSN) == "" {
		return fail(ExitUsage, "--sample-hash exige AUDITOR_RESTORE_VERIFY_SOURCE_DSN ou _FILE")
	}
	if opt.BackupPath == "" || opt.EnvironmentID == "" || opt.Image == "" {
		return fail(ExitUsage, "informe --backup, --environment e --image")
	}
	if !opt.ConfirmIsolated {
		return fail(ExitSecurity, "confirmação de isolamento ausente (--confirm-isolated-target)")
	}
	rt := opt.RuntimeExec
	if rt == nil {
		var err error
		rt, err = NewRuntime(opt.Runtime)
		if err != nil {
			return fail(ExitUsage, err.Error())
		}
	}
	if err := rt.Available(ctx); err != nil {
		return fail(ExitUsage, err.Error())
	}
	ephemeralFP := FingerprintHostPortDB("127.0.0.1", "5432", "restore")
	preStart := time.Now()
	pre, err := Prevalidate(PrevalidateInput{
		BackupPath: opt.BackupPath, ExpectedSHA: opt.ExpectedSHA256, Image: opt.Image,
		SourceMajor: opt.SourceMajor, ConfirmIsolated: opt.ConfirmIsolated,
		Fingerprints: opt.TargetFingerprints, CandidateFP: ephemeralFP, RequiredBytes: opt.DiskBytesRequired,
	})
	preSecs := int(time.Since(preStart).Seconds())
	ev.PrevalidateSeconds = &preSecs
	if err != nil {
		msg := err.Error()
		code := ExitUsage
		ev.ArtifactSHA256 = safeSHA(opt.BackupPath)
		switch {
		case strings.Contains(msg, "sha256 do artefato diverge"), strings.Contains(msg, "ilegível"):
			code = ExitBlocked
		case strings.Contains(msg, "isolamento"), strings.Contains(msg, "fingerprint"):
			code = ExitSecurity
		}
		return fail(code, msg)
	}
	ev.ArtifactSHA256 = pre.SHA256
	ephem, err := NewEphemeral(rt, opt.Image, "")
	if err != nil {
		return fail(ExitUsage, err.Error())
	}
	defer func() {
		if !opt.SkipContainerCleanup {
			ephem.Destroy(context.WithoutCancel(ctx))
			_ = os.RemoveAll(ephem.WorkDir)
		}
	}()
	if _, err := ephem.StageBackup(opt.BackupPath); err != nil {
		return fail(ExitUsage, "preparar backup: "+err.Error())
	}
	containerBackup := ephem.BackupMountPath(opt.BackupPath)
	if err := ephem.Start(ctx); err != nil {
		return fail(ExitBlocked, "container: "+err.Error())
	}
	if err := ListArchive(ctx, ephem, containerBackup); err != nil {
		return fail(ExitBlocked, err.Error())
	}
	rtoStart := time.Now()
	_, stderr, err := ephem.Exec(ctx, "pg_restore", "--clean", "--if-exists", "--no-owner", "--no-acl", "--exit-on-error", "-U", "restore", "-d", "restore", containerBackup)
	if err != nil {
		rto := int(time.Since(rtoStart).Seconds())
		ev.RTOSeconds = &rto
		return fail(ExitBlocked, "restauração falhou: "+strings.TrimSpace(stderr))
	}
	if _, _, err := ephem.ExecSQL(ctx, "SELECT count(*) FROM pg_catalog.pg_class"); err != nil {
		rto := int(time.Since(rtoStart).Seconds())
		ev.RTOSeconds = &rto
		return fail(ExitBlocked, "banco restaurado inacessível")
	}
	rto := int(time.Since(rtoStart).Seconds())
	ev.RTOSeconds = &rto
	verifyStart := time.Now()
	estimates := MarkComparable(opt.EstimatedRows, opt.WriteCounters)
	inv, invDiv, invErr := VerifyInventory(ctx, ephem, opt.Inventory)
	ev.Checks = append(ev.Checks, inv)
	ev.Divergences = append(ev.Divergences, invDiv...)
	if invErr != nil && inv.Status == StatusFailed {
		verifySecs := int(time.Since(verifyStart).Seconds())
		ev.VerifySeconds = &verifySecs
		return fail(ExitBlocked, "inventário: "+invErr.Error())
	}
	cnt, cntDiv, _ := VerifyCounts(ctx, ephem, estimates)
	ev.Checks = append(ev.Checks, cnt)
	ev.Divergences = append(ev.Divergences, cntDiv...)
	hashOut := VerifySampleHashOptIn(ctx, ephem, opt.SampleHash, opt.SourceDSN, estimates, opt.WriteCounters)
	ev.Checks = append(ev.Checks, hashOut.Check)
	ev.Divergences = append(ev.Divergences, hashOut.Divergences...)
	if hashOut.SourceRefusedWrite {
		verifySecs := int(time.Since(verifyStart).Seconds())
		ev.VerifySeconds = &verifySecs
		return fail(ExitSecurity, hashOut.Check.Detail)
	}
	am, amDiv, amErr := VerifyAmcheck(ctx, ephem)
	ev.Checks = append(ev.Checks, am)
	ev.Divergences = append(ev.Divergences, amDiv...)
	if amErr != nil && am.Status == StatusFailed {
		verifySecs := int(time.Since(verifyStart).Seconds())
		ev.VerifySeconds = &verifySecs
		return fail(ExitBlocked, "amcheck: "+amErr.Error())
	}
	verifySecs := int(time.Since(verifyStart).Seconds())
	ev.VerifySeconds = &verifySecs
	physicalFail := am.Status == StatusFailed || am.Status == StatusDiverged
	ev.Succeeded = !physicalFail && inv.Status != StatusFailed && len(ev.Divergences) == 0
	code := ExitOK
	if !ev.Succeeded {
		code = ExitBlocked
	}
	if hashOut.SourceUnavailable {
		code = ExitInsufficient
	}
	return Result{ExitCode: code, Evidence: ev, Summary: RenderSummary(ev)}
}

func safeSHA(path string) string {
	sha, _, err := FileSHA256(path)
	if err != nil {
		return ""
	}
	return sha
}
