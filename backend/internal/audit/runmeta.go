package audit

import "context"

type runMetaKey struct{}

// RunMeta is attached to the context for the duration of an audit run.
type RunMeta struct {
	EnvironmentID string
	AuditRunID    string
}

// WithRunMeta stores environment and audit run identifiers on ctx.
func WithRunMeta(ctx context.Context, meta RunMeta) context.Context {
	return context.WithValue(ctx, runMetaKey{}, meta)
}

// RunMetaFromContext returns run metadata if present.
func RunMetaFromContext(ctx context.Context) (RunMeta, bool) {
	v, ok := ctx.Value(runMetaKey{}).(RunMeta)
	return v, ok
}
