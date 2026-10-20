package loganomaly

import (
	"regexp"
	"strings"

	"github.com/osmendes/db-auditor/internal/fingerprint"
)

// Mask token used in templates (Drain-style wildcard).
const MaskToken = "<*>"

var (
	reEmail = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	// IPv4 first; IPv6 compressed/full (avoid eating too much).
	reIPv4 = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d?\d)\b`)
	reIPv6 = regexp.MustCompile(`(?i)\b(?:(?:[0-9a-f]{1,4}:){7}[0-9a-f]{1,4}|(?:[0-9a-f]{1,4}:){1,7}:|(?:[0-9a-f]{1,4}:){1,6}:[0-9a-f]{1,4}|(?:[0-9a-f]{1,4}:){1,5}(?::[0-9a-f]{1,4}){1,2}|(?:[0-9a-f]{1,4}:){1,4}(?::[0-9a-f]{1,4}){1,3}|(?:[0-9a-f]{1,4}:){1,3}(?::[0-9a-f]{1,4}){1,4}|(?:[0-9a-f]{1,4}:){1,2}(?::[0-9a-f]{1,4}){1,5}|[0-9a-f]{1,4}:(?::[0-9a-f]{1,4}){1,6}|:(?::[0-9a-f]{1,4}){1,7})\b`)
	reUUID = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	reHash = regexp.MustCompile(`\b[0-9a-fA-F]{32,64}\b`)
	// Unix or Windows-ish paths (conservative).
	reUnixPath = regexp.MustCompile(`(?:^|[\s="'(:,])(/[A-Za-z0-9._\-]+(?:/[A-Za-z0-9._\-]+)+)`)
	reWinPath  = regexp.MustCompile(`(?i)\b[A-Z]:\\(?:[^\s"'<>|]+\\)*[^\s"'<>|]+`)
	// user=/database=/application_name= parameters (keep key, mask value).
	reKVParam = regexp.MustCompile(`(?i)\b(user|database|dbname|application_name|app|host|remote_host|password|pwd)=([^\s,]+)`)
	reNumber  = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	reQuoted  = regexp.MustCompile(`'(?:''|[^'])*'|"(?:\\.|[^"\\])*"`)
	reDollar  = regexp.MustCompile(`(?s)\$[A-Za-z_0-9]*\$.*?\$[A-Za-z_0-9]*\$`)
	reSpace   = regexp.MustCompile(`\s+`)
)

// MaskMessage replaces variable parts with MaskToken (RNF-02, ADR-0001 D16).
// Starts from fingerprint literal stripping patterns, then adds IP/email/UUID/path/params.
// Never returns the original message; caller must discard the raw input.
func MaskMessage(msg string) string {
	if msg == "" {
		return ""
	}
	q := msg
	// Order: structured secrets first, then SQL-ish literals, then free-form PII.
	q = reDollar.ReplaceAllString(q, MaskToken)
	q = reQuoted.ReplaceAllString(q, MaskToken)
	q = reEmail.ReplaceAllString(q, MaskToken)
	q = reUUID.ReplaceAllString(q, MaskToken)
	q = reIPv6.ReplaceAllString(q, MaskToken)
	q = reIPv4.ReplaceAllString(q, MaskToken)
	q = reHash.ReplaceAllString(q, MaskToken)
	q = reWinPath.ReplaceAllString(q, MaskToken)
	q = reUnixPath.ReplaceAllStringFunc(q, func(m string) string {
		// Preserve leading delimiter when present.
		if len(m) > 0 && (m[0] == ' ' || m[0] == '=' || m[0] == '"' || m[0] == '\'' || m[0] == '(' || m[0] == ':' || m[0] == ',') {
			return string(m[0]) + MaskToken
		}
		return MaskToken
	})
	q = reKVParam.ReplaceAllString(q, "${1}="+MaskToken)
	q = reNumber.ReplaceAllString(q, MaskToken)
	// Also apply fingerprint strip for any residual SQL comments/literals shape,
	// then re-normalize '?' from StripLiterals into MaskToken.
	stripped := fingerprint.StripLiterals(q)
	stripped = strings.ReplaceAll(stripped, "?", MaskToken)
	stripped = reSpace.ReplaceAllString(stripped, " ")
	return strings.TrimSpace(stripped)
}

// HasFixedToken reports whether the masked template has any non-mask token (RNF-02).
func HasFixedToken(masked string) bool {
	for _, tok := range strings.Fields(masked) {
		if tok != MaskToken && !strings.Contains(tok, MaskToken) {
			// token may be "user=<*>" — still has fixed "user="
			fixed := strings.ReplaceAll(tok, MaskToken, "")
			fixed = strings.Trim(fixed, `="'(),.;:[]{}`)
			if fixed != "" {
				return true
			}
		}
		if strings.Contains(tok, MaskToken) {
			prefix := strings.Split(tok, MaskToken)[0]
			if strings.Trim(prefix, `="'(),.;:[]{}`) != "" {
				return true
			}
		}
	}
	return false
}

// ContainsSensitiveLeak is a test helper: true if any needle survives in haystack.
func ContainsSensitiveLeak(haystack string, needles ...string) bool {
	h := strings.ToLower(haystack)
	for _, n := range needles {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if strings.Contains(h, strings.ToLower(n)) {
			return true
		}
	}
	return false
}
