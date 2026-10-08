package api

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPOnlyTrustsConfiguredProxy(t *testing.T) {
	t.Setenv("AUDITOR_TRUSTED_PROXY_CIDRS", "192.0.2.0/24")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:4321"
	r.Header.Set("X-Forwarded-For", "198.51.100.8")
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := clientIP(r); got != "203.0.113.9" || secureRequest(r) {
		t.Fatalf("untrusted proxy: ip=%q secure=%t", got, secureRequest(r))
	}
	r.RemoteAddr = "192.0.2.10:4321"
	r.Header.Set("X-Forwarded-For", "10.0.0.2, 198.51.100.8")
	if got := clientIP(r); got != "198.51.100.8" || !secureRequest(r) {
		t.Fatalf("trusted proxy: ip=%q secure=%t", got, secureRequest(r))
	}
	r.Header.Set("X-Forwarded-For", "bad-ip")
	if got := clientIP(r); got != "192.0.2.10" {
		t.Fatalf("malformed forwarded header: %q", got)
	}
}
