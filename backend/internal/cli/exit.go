package cli

// Exit codes shared by auditor CLI subcommands (specs 01, 03, 08, 10).
const (
	ExitOK              = 0 // nothing above the fail-on threshold
	ExitBlocked         = 1 // gate failed (severity at or above fail-on)
	ExitUsage           = 2 // bad usage, invalid credential, or config
	ExitInsufficient    = 3 // parse error, partial/stale/missing context
	ExitSecurityRefused = 4 // security refusal (e.g. writable target role)
)

// ExitCodeName is a stable label for tests and docs.
func ExitCodeName(code int) string {
	switch code {
	case ExitOK:
		return "ok"
	case ExitBlocked:
		return "blocked"
	case ExitUsage:
		return "usage"
	case ExitInsufficient:
		return "insufficient"
	case ExitSecurityRefused:
		return "security_refused"
	default:
		return "unknown"
	}
}
