package buildinfo

// Set via -ldflags -X at build/image time. Rule catalog versions stay independent.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string {
	if Commit == "" || Commit == "unknown" {
		return Version
	}
	short := Commit
	if len(short) > 12 {
		short = short[:12]
	}
	return Version + "+" + short
}
