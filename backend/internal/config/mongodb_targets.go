package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// LoadMongoTargetURIs reads explicitly configured MongoDB targets. Each URI
// names one database; the adapter never enumerates other databases.
func LoadMongoTargetURIs() map[string]string {
	out := map[string]string{}
	for n := 1; n <= maxTargetSlots; n++ {
		prefix := fmt.Sprintf("AUDITOR_TARGET_%d_", n)
		if !strings.EqualFold(envTrim(prefix+"ENGINE"), "mongodb") {
			continue
		}
		id := strings.ToLower(envTrim(prefix + "ENVIRONMENT_ID"))
		host := envTrim(prefix + "HOST")
		if id == "" || host == "" {
			continue
		}
		port := envTrim(prefix + "PORT")
		if port == "" {
			port = "27017"
		}
		u := &url.URL{Scheme: "mongodb", Host: net.JoinHostPort(host, port), Path: "/" + envTrim(prefix+"DATABASE")}
		user := envTrim(prefix + "USER")
		if password := os.Getenv(prefix + "PASSWORD"); password != "" {
			u.User = url.UserPassword(user, password)
		} else {
			u.User = url.User(user)
		}
		q := u.Query()
		q.Set("tls", "true")
		q.Set("directConnection", "true")
		u.RawQuery = q.Encode()
		out[id] = u.String()
	}
	const legacy = "AUDITOR_MONGODB_URI_"
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(key, legacy) || value == "" {
			continue
		}
		id := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(key, legacy), "_", "-"))
		out[id] = value
	}
	return out
}

// ValidateMongoTargetURIs refuses discovery of additional hosts, weak TLS
// settings, and targets outside the same explicit hostname allowlist as SQL.
func ValidateMongoTargetURIs(targets map[string]string) error {
	if len(targets) == 0 {
		return nil
	}
	allowed := splitCSV(os.Getenv("AUDITOR_TARGET_ALLOWED_HOSTS"))
	if len(allowed) == 0 {
		return fmt.Errorf("AUDITOR_TARGET_ALLOWED_HOSTS is required for MongoDB targets")
	}
	for id, raw := range targets {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "mongodb" || u.User == nil || u.Hostname() == "" || strings.Contains(u.Host, ",") || strings.Trim(u.Path, "/") == "" {
			return fmt.Errorf("invalid MongoDB target URI for environment %s", id)
		}
		if _, err := strconv.Atoi(u.Port()); err != nil {
			return fmt.Errorf("MongoDB target port is required for environment %s", id)
		}
		host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
		approved := false
		for _, name := range allowed {
			if strings.EqualFold(strings.TrimSuffix(name, "."), host) {
				approved = true
				break
			}
		}
		if !approved {
			return fmt.Errorf("MongoDB target host is not allowlisted for environment %s", id)
		}
		q := u.Query()
		if !strings.EqualFold(q.Get("directConnection"), "true") || q.Get("tlsInsecure") != "" || q.Get("tlsAllowInvalidCertificates") != "" || q.Get("tlsAllowInvalidHostnames") != "" {
			return fmt.Errorf("MongoDB target requires direct connection and verified TLS for environment %s", id)
		}
		if !strings.EqualFold(q.Get("tls"), "true") {
			loopback := net.ParseIP(host)
			local := host == "localhost" || loopback != nil && loopback.IsLoopback()
			if !local || os.Getenv("AUDITOR_ALLOW_INSECURE_LOCAL_TARGETS") != "true" || !strings.EqualFold(q.Get("tls"), "false") {
				return fmt.Errorf("MongoDB target requires verified TLS for environment %s", id)
			}
		}
	}
	return nil
}
