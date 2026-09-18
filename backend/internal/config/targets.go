package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const maxTargetSlots = 20

// TargetDSNKey builds the legacy env var name for a full connection string.
// Prefer discrete AUDITOR_TARGET_N_* fields; this remains as a fallback.
// Example: 00000000-0000-0000-0000-000000000001
// → AUDITOR_TARGET_DSN_00000000_0000_0000_0000_000000000001
func TargetDSNKey(environmentID string) string {
	id := strings.ReplaceAll(strings.TrimSpace(environmentID), "-", "_")
	return "AUDITOR_TARGET_DSN_" + strings.ToUpper(id)
}

// TargetSlotHint returns a human-readable hint for configuring an environment.
func TargetSlotHint(environmentID string) string {
	id := strings.ToLower(strings.TrimSpace(environmentID))
	return fmt.Sprintf(
		"AUDITOR_TARGET_N_ENVIRONMENT_ID=%s + HOST/PORT/DATABASE/USER/PASSWORD (N=1..%d)",
		id, maxTargetSlots,
	)
}

// BuildTargetDSN assembles a PostgreSQL URI from discrete fields.
// Empty port defaults to 5432; empty sslmode defaults to require.
func BuildTargetDSN(host, port, database, user, password, sslmode string) (string, error) {
	host = strings.TrimSpace(host)
	database = strings.TrimSpace(database)
	user = strings.TrimSpace(user)
	port = strings.TrimSpace(port)
	sslmode = strings.TrimSpace(sslmode)
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	if database == "" {
		return "", fmt.Errorf("database is required")
	}
	if user == "" {
		return "", fmt.Errorf("user is required")
	}
	if port == "" {
		port = "5432"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("port must be numeric")
	}
	if sslmode == "" {
		sslmode = "require"
	}
	u := &url.URL{
		Scheme: "postgresql",
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + database,
	}
	if password != "" {
		u.User = url.UserPassword(user, password)
	} else {
		u.User = url.User(user)
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func envTrim(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// loadDiscreteTargetDSNs reads AUDITOR_TARGET_N_* slots (N=1..maxTargetSlots).
// Required per slot: ENVIRONMENT_ID, HOST, DATABASE, USER.
// Optional: PORT (default 5432), PASSWORD, SSLMODE (default require).
func loadDiscreteTargetDSNs() map[string]string {
	out := make(map[string]string)
	for n := 1; n <= maxTargetSlots; n++ {
		prefix := fmt.Sprintf("AUDITOR_TARGET_%d_", n)
		envID := strings.ToLower(envTrim(prefix + "ENVIRONMENT_ID"))
		host := envTrim(prefix + "HOST")
		if envID == "" || host == "" {
			continue
		}
		dsn, err := BuildTargetDSN(
			host,
			envTrim(prefix+"PORT"),
			envTrim(prefix+"DATABASE"),
			envTrim(prefix+"USER"),
			os.Getenv(prefix+"PASSWORD"), // keep internal spaces if any
			envTrim(prefix+"SSLMODE"),
		)
		if err != nil {
			continue
		}
		out[envID] = dsn
	}
	return out
}

// loadLegacyTargetDSNs reads AUDITOR_TARGET_DSN_<uuid> full connection strings.
func loadLegacyTargetDSNs() map[string]string {
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

// LoadTargetDSNs builds environment_id → DSN from discrete slots first, then
// legacy AUDITOR_TARGET_DSN_* (discrete wins on conflict).
func LoadTargetDSNs() map[string]string {
	out := loadLegacyTargetDSNs()
	for id, dsn := range loadDiscreteTargetDSNs() {
		out[id] = dsn
	}
	return out
}

// DSNForEnvironment returns the configured target DSN, if any.
func DSNForEnvironment(targets map[string]string, environmentID string) string {
	id := strings.ToLower(strings.TrimSpace(environmentID))
	if targets != nil {
		if dsn, ok := targets[id]; ok && dsn != "" {
			return dsn
		}
	}
	// Rebuild from env (covers nil map and vars set after process start in tests).
	m := LoadTargetDSNs()
	if dsn, ok := m[id]; ok {
		return dsn
	}
	return strings.TrimSpace(os.Getenv(TargetDSNKey(environmentID)))
}
