package timescale

import (
	"fmt"
	"strconv"
	"strings"
)

// MinSupportedMajor is the lowest TimescaleDB major version fully supported
// by this collector set. Older versions are marked SKIPPED_UNSUPPORTED.
const MinSupportedMajor = 2

// ParseVersion extracts major.minor.patch from an extension version string
// such as "2.17.2" or "2.14.2-dev". Non-numeric suffixes are ignored.
func ParseVersion(version string) (major, minor, patch int, err error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return 0, 0, 0, fmt.Errorf("empty version")
	}
	// Drop build metadata / suffix after first non [0-9.].
	base := version
	for i, r := range version {
		if (r < '0' || r > '9') && r != '.' {
			base = version[:i]
			break
		}
	}
	parts := strings.Split(base, ".")
	if len(parts) == 0 || parts[0] == "" {
		return 0, 0, 0, fmt.Errorf("invalid version %q", version)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("major in %q: %w", version, err)
	}
	if len(parts) > 1 && parts[1] != "" {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 && parts[2] != "" {
		patch, _ = strconv.Atoi(parts[2])
	}
	return major, minor, patch, nil
}

// EvaluateCompatibility returns whether the installed version is supported and a note.
func EvaluateCompatibility(major int, version string) (compatible bool, note string) {
	if major < MinSupportedMajor {
		return false, fmt.Sprintf(
			"TimescaleDB %s (major %d) is below minimum supported major %d",
			version, major, MinSupportedMajor,
		)
	}
	return true, ""
}
