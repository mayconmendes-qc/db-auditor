package api

import (
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

// clientIP only accepts forwarding headers from explicitly trusted reverse
// proxies. Walk the chain from the nearest hop so a client-supplied prefix
// cannot select an arbitrary identity for rate limiting.
func clientIP(r *http.Request) string {
	remote := remoteIP(r.RemoteAddr)
	if !trustedProxy(remote) {
		return remote
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		candidate, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil {
			return remote
		}
		if !trustedProxy(candidate.String()) {
			return candidate.String()
		}
	}
	return remote
}

func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}
	return addr
}

func trustedProxy(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, raw := range strings.Split(os.Getenv("AUDITOR_TRUSTED_PROXY_CIDRS"), ",") {
		prefix, parseErr := netip.ParsePrefix(strings.TrimSpace(raw))
		if parseErr == nil && prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || trustedProxy(remoteIP(r.RemoteAddr)) &&
		strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
