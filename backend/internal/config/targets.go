package config

import (
	"os"
	"strings"
)

// TargetDSNKey builds the env var name for an environment UUID.
// Example: 00000000-0000-0000-0000-000000000001
// → AUDITOR_TARGET_DSN_00000000_0000_0000_0000_000000000001
func TargetDSNKey(environmentID string) string {
	id := strings.ReplaceAll(strings.TrimSpace(environmentID), "-", "_")
	return "AUDITOR_TARGET_DSN_" + strings.ToUpper(id)
}

// LoadTargetDSNs reads all AUDITOR_TARGET_DSN_* variables into a map keyed by UUID.
func LoadTargetDSNs() map[string]string {
	out := make(map[string]string)
	const prefix = "AUDITOR_TARGET_DSN_"
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], strings.TrimSpace(parts[1])
		if !strings.HasPrefix(key, prefix) || val == "" {
			continue
		}
		suffix := strings.TrimPrefix(key, prefix)
		uuid := strings.ToLower(strings.ReplaceAll(suffix, "_", "-"))
		out[uuid] = val
	}
	return out
}

// DSNForEnvironment returns the configured target DSN, if any.
func DSNForEnvironment(targets map[string]string, environmentID string) string {
	id := strings.ToLower(strings.TrimSpace(environmentID))
	if targets != nil {
		if dsn, ok := targets[id]; ok {
			return dsn
		}
	}
	// Direct env lookup (covers nil map and keys set after LoadTargetDSNs).
	return strings.TrimSpace(os.Getenv(TargetDSNKey(environmentID)))
}
