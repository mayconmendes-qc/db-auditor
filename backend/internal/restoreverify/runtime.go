package restoreverify

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runtime interface {
	Name() string
	Available(ctx context.Context) error
	Run(ctx context.Context, args ...string) (stdout, stderr string, err error)
}

type ExecRuntime struct{ Binary string }

func NewRuntime(preferred string) (Runtime, error) {
	preferred = strings.ToLower(strings.TrimSpace(preferred))
	var candidates []string
	switch preferred {
	case "docker":
		candidates = []string{"docker"}
	case "podman":
		candidates = []string{"podman"}
	case "", "auto":
		candidates = []string{"podman", "docker"}
	default:
		return nil, fmt.Errorf("runtime inválido %q (use docker ou podman)", preferred)
	}
	for _, bin := range candidates {
		if path, err := exec.LookPath(bin); err == nil {
			return &ExecRuntime{Binary: path}, nil
		}
	}
	return nil, fmt.Errorf("runtime de container ausente (%s)", strings.Join(candidates, "/"))
}

func (r *ExecRuntime) Name() string {
	base := r.Binary
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return base
}

func (r *ExecRuntime) Available(ctx context.Context) error {
	if _, _, err := r.Run(ctx, "info"); err != nil {
		return fmt.Errorf("runtime %s indisponível: %w", r.Name(), err)
	}
	return nil
}

func (r *ExecRuntime) Run(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, r.Binary, args...) //nolint:gosec // G204: Binary is podman/docker chosen by the operator; args are drill-controlled.
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

type FakeRuntime struct {
	Bin       string
	Calls     [][]string
	Responses map[string]struct {
		Out, Err string
		CodeErr  error
	}
	FailInfo bool
}

func (f *FakeRuntime) Name() string {
	if f.Bin == "" {
		return "fake"
	}
	return f.Bin
}

func (f *FakeRuntime) Available(context.Context) error {
	if f.FailInfo {
		return fmt.Errorf("runtime ausente")
	}
	return nil
}

func (f *FakeRuntime) Run(_ context.Context, args ...string) (string, string, error) {
	f.Calls = append(f.Calls, append([]string{}, args...))
	key := strings.Join(args, " ")
	if f.Responses != nil {
		if resp, ok := f.Responses[key]; ok {
			return resp.Out, resp.Err, resp.CodeErr
		}
		for prefix, resp := range f.Responses {
			if strings.HasPrefix(key, prefix) {
				return resp.Out, resp.Err, resp.CodeErr
			}
		}
	}
	return "", "", nil
}
