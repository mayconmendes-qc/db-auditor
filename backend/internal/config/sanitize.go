package config

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	// password in URI userinfo: scheme://user:password@host
	uriUserPass = regexp.MustCompile(`(?i)(://[^:/?#\s]+):([^@/?#\s]+)@`)
	// common key=value password fragments in error text
	passwordKV = regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*\S+`)
)

// SanitizeDSN redacts credentials from a PostgreSQL connection URI or any string that may embed one.
// Safe for logs, PartialError messages, and API error envelopes.
func SanitizeDSN(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" && u.User != nil {
		name := u.User.Username()
		if _, has := u.User.Password(); has {
			// Rebuild without relying on UserPassword encoding quirks.
			u.User = nil
			redacted := u.String()
			// Insert user:***@ after scheme://
			if i := strings.Index(redacted, "://"); i >= 0 {
				redacted = redacted[:i+3] + name + ":***@" + redacted[i+3:]
			}
			return redacted
		}
	}
	// Fallback for free-form error strings that embed URIs or password=...
	out := uriUserPass.ReplaceAllString(s, `${1}:***@`)
	out = passwordKV.ReplaceAllString(out, `${1}=***`)
	return out
}

// SanitizeError returns err.Error() with credentials redacted.
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeDSN(err.Error())
}
