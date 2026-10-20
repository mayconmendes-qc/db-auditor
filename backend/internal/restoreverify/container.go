package restoreverify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const runLabelPrefix = "db-auditor.run="

type Ephemeral struct {
	Runtime   Runtime
	Image     string
	RunID     string
	Name      string
	Password  string
	Volume    string
	WorkDir   string
	started   bool
	DSNInside string
}

func NewEphemeral(rt Runtime, image, workDir string) (*Ephemeral, error) {
	if image == "" {
		return nil, fmt.Errorf("imagem PostgreSQL obrigatória (--image)")
	}
	id, err := randomHex(8)
	if err != nil {
		return nil, err
	}
	pass, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	if workDir == "" {
		workDir, err = os.MkdirTemp("", "db-auditor-restore-*")
		if err != nil {
			return nil, err
		}
	}
	return &Ephemeral{
		Runtime: rt, Image: image, RunID: id,
		Name: "db-auditor-restore-" + id, Password: pass,
		Volume: "db-auditor-restore-vol-" + id, WorkDir: workDir,
	}, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (e *Ephemeral) Label() string { return runLabelPrefix + e.RunID }

func SweepOrphans(ctx context.Context, rt Runtime) {
	out, _, err := rt.Run(ctx, "ps", "-aq", "--filter", "label=db-auditor.run")
	if err == nil {
		for _, id := range strings.Fields(out) {
			_, _, _ = rt.Run(ctx, "rm", "-f", id)
		}
	}
	vout, _, err := rt.Run(ctx, "volume", "ls", "-q", "--filter", "label=db-auditor.run")
	if err == nil {
		for _, id := range strings.Fields(vout) {
			_, _, _ = rt.Run(ctx, "volume", "rm", "-f", id)
		}
	}
}

func (e *Ephemeral) Start(ctx context.Context) error {
	SweepOrphans(ctx, e.Runtime)
	if _, _, err := e.Runtime.Run(ctx, "volume", "create", "--label", e.Label(), e.Volume); err != nil {
		return fmt.Errorf("criar volume: %w", err)
	}
	args := []string{"run", "-d", "--name", e.Name, "--label", e.Label(), "--network", "none",
		"-e", "POSTGRES_PASSWORD=" + e.Password, "-e", "POSTGRES_USER=restore", "-e", "POSTGRES_DB=restore",
		"-v", e.Volume + ":/var/lib/postgresql/data", "-v", e.WorkDir + ":/backup:ro", e.Image}
	_, stderr, err := e.Runtime.Run(ctx, args...)
	if err != nil {
		_, _, _ = e.Runtime.Run(ctx, "volume", "rm", "-f", e.Volume)
		return fmt.Errorf("criar container: %v %s", err, stderr)
	}
	e.started = true
	e.DSNInside = fmt.Sprintf("postgresql://restore:%s@127.0.0.1:5432/restore?sslmode=disable", e.Password)
	return e.waitReady(ctx, 90*time.Second)
}

func (e *Ephemeral) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, _, err := e.Runtime.Run(ctx, "exec", e.Name, "pg_isready", "-U", "restore", "-d", "restore"); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("postgres no container não ficou pronto a tempo")
}

func (e *Ephemeral) Exec(ctx context.Context, args ...string) (string, string, error) {
	return e.Runtime.Run(ctx, append([]string{"exec", e.Name}, args...)...)
}

func (e *Ephemeral) ExecSQL(ctx context.Context, sql string) (string, string, error) {
	return e.Exec(ctx, "psql", "-U", "restore", "-d", "restore", "-v", "ON_ERROR_STOP=1", "-At", "-c", sql)
}

func (e *Ephemeral) Destroy(ctx context.Context) {
	if e == nil || e.Runtime == nil {
		return
	}
	if e.Name != "" {
		_, _, _ = e.Runtime.Run(ctx, "rm", "-f", e.Name)
	}
	if e.Volume != "" {
		_, _, _ = e.Runtime.Run(ctx, "volume", "rm", "-f", e.Volume)
	}
	e.started = false
}

func (e *Ephemeral) BackupMountPath(hostPath string) string {
	return "/backup/" + filepath.Base(hostPath)
}

func (e *Ephemeral) StageBackup(hostPath string) (string, error) {
	info, err := os.Stat(hostPath)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(e.WorkDir, filepath.Base(hostPath))
	if info.IsDir() {
		if err := os.Symlink(hostPath, dst); err != nil {
			return "", copyDir(hostPath, dst)
		}
		return dst, nil
	}
	if err := os.Symlink(hostPath, dst); err != nil {
		data, rerr := os.ReadFile(hostPath) //nolint:gosec
		if rerr != nil {
			return "", rerr
		}
		if werr := os.WriteFile(dst, data, 0o600); werr != nil { //nolint:gosec // G703: dst is under Ephemeral.WorkDir with filepath.Base.
			return "", werr
		}
	}
	return dst, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := os.ReadFile(path) //nolint:gosec
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600) //nolint:gosec // G703: target is under dst joined with a path relative to src.
	})
}
