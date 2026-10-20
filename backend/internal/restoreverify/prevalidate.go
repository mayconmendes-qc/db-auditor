package restoreverify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ImageMajor(image string) (int, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return 0, fmt.Errorf("imagem vazia")
	}
	tag := image
	if i := strings.LastIndex(image, ":"); i >= 0 {
		tag = image[i+1:]
	}
	tag = strings.Split(tag, "-")[0]
	m := regexp.MustCompile(`^(\d+)`).FindStringSubmatch(tag)
	if m == nil {
		return 0, fmt.Errorf("não foi possível extrair a major de %q", image)
	}
	n, _ := strconv.Atoi(m[1])
	if n < 10 {
		return 0, fmt.Errorf("major inválida %d", n)
	}
	return n, nil
}

func FileSHA256(path string) (string, int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	var size int64
	if info.IsDir() {
		err = filepath.Walk(path, func(p string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if fi.IsDir() {
				return nil
			}
			f, err := os.Open(p) //nolint:gosec
			if err != nil {
				return err
			}
			defer f.Close()
			n, err := io.Copy(h, f)
			size += n
			return err
		})
		if err != nil {
			return "", 0, err
		}
	} else {
		f, err := os.Open(path) //nolint:gosec
		if err != nil {
			return "", 0, err
		}
		defer f.Close()
		n, err := io.Copy(h, f)
		if err != nil {
			return "", 0, err
		}
		size = n
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func ValidateExpectedSHA(got, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return nil
	}
	if !sha256Hex.MatchString(expected) {
		return fmt.Errorf("sha256 esperado inválido")
	}
	if got != expected {
		return fmt.Errorf("sha256 do artefato diverge do informado")
	}
	return nil
}

func FreeDiskBytes(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil //nolint:gosec // G115: free disk bytes fit int64 for drill sizing checks.
}

type PrevalidateInput struct {
	BackupPath      string
	ExpectedSHA     string
	Image           string
	SourceMajor     int
	ConfirmIsolated bool
	Fingerprints    []string
	CandidateFP     string
	RequiredBytes   int64
}

type PrevalidateResult struct {
	SHA256    string
	SizeBytes int64
	FreeBytes int64
	ImageMaj  int
}

func Prevalidate(in PrevalidateInput) (*PrevalidateResult, error) {
	if !in.ConfirmIsolated {
		return nil, fmt.Errorf("confirmação de isolamento ausente (--confirm-isolated-target)")
	}
	if MatchesAnyFingerprint(in.CandidateFP, in.Fingerprints) {
		return nil, fmt.Errorf("destino coincide com fingerprint de ambiente configurado")
	}
	if _, err := os.Stat(in.BackupPath); err != nil {
		return nil, fmt.Errorf("backup ilegível: %w", err)
	}
	sha, size, err := FileSHA256(in.BackupPath)
	if err != nil {
		return nil, fmt.Errorf("checksum do backup: %w", err)
	}
	if err := ValidateExpectedSHA(sha, in.ExpectedSHA); err != nil {
		return nil, err
	}
	maj, err := ImageMajor(in.Image)
	if err != nil {
		return nil, err
	}
	if in.SourceMajor > 0 && maj < in.SourceMajor {
		return nil, fmt.Errorf("major da imagem (%d) menor que a origem (%d)", maj, in.SourceMajor)
	}
	need := in.RequiredBytes
	if need <= 0 {
		need = size * 2
	}
	free, err := FreeDiskBytes(filepath.Dir(in.BackupPath))
	if err != nil {
		free, err = FreeDiskBytes(".")
		if err != nil {
			return nil, fmt.Errorf("disco: %w", err)
		}
	}
	if free < need {
		return nil, fmt.Errorf("disco insuficiente: livre %d < necessário %d", free, need)
	}
	return &PrevalidateResult{SHA256: sha, SizeBytes: size, FreeBytes: free, ImageMaj: maj}, nil
}

func ListArchive(ctx context.Context, e *Ephemeral, containerPath string) error {
	_, stderr, err := e.Exec(ctx, "pg_restore", "--list", containerPath)
	if err != nil {
		return fmt.Errorf("arquivo ilegível (pg_restore --list): %v %s", err, stderr)
	}
	return nil
}
