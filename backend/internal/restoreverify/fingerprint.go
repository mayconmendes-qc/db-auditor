package restoreverify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strings"
)

func FingerprintDSN(dsn string) string {
	host, port, database, err := parseHostPortDB(dsn)
	if err != nil || host == "" || database == "" {
		return ""
	}
	if port == "" {
		port = "5432"
	}
	raw := strings.ToLower(host) + ":" + port + "/" + strings.ToLower(database)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func FingerprintHostPortDB(host, port, database string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	port = strings.TrimSpace(port)
	database = strings.ToLower(strings.TrimSpace(database))
	if host == "" || database == "" {
		return ""
	}
	if port == "" {
		port = "5432"
	}
	raw := host + ":" + port + "/" + database
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func parseHostPortDB(dsn string) (host, port, database string, err error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return "", "", "", fmt.Errorf("empty dsn")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", "", "", err
	}
	host = u.Hostname()
	port = u.Port()
	database = strings.TrimPrefix(u.Path, "/")
	if i := strings.IndexByte(database, '?'); i >= 0 {
		database = database[:i]
	}
	if host == "" && u.Host != "" {
		h, p, e := net.SplitHostPort(u.Host)
		if e == nil {
			host, port = h, p
		} else {
			host = u.Host
		}
	}
	return host, port, database, nil
}

func MatchesAnyFingerprint(candidate string, configured []string) bool {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if candidate == "" {
		return false
	}
	for _, fp := range configured {
		if strings.EqualFold(strings.TrimSpace(fp), candidate) {
			return true
		}
	}
	return false
}
