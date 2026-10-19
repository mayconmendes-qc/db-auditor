package cli

import (
	"fmt"
	"os"
)

// Run is the CLI entry used by cmd/auditor before config.Load().
// It returns an exit code. Unknown commands return ExitUsage.
func Run(args []string) int {
	if len(args) == 0 || IsHelpArg(args) {
		WriteHelp(os.Stdout)
		return ExitOK
	}
	if err := RefuseSecretFlags(args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}
	switch args[0] {
	case "migration-risk":
		return runMigrationRisk(args[1:])
	case "log-anomaly":
		return runLogAnomaly(args[1:])
	case "dq-contracts":
		return runDQContracts(args[1:])
	case "restore-verify":
		return runRestoreVerify(args[1:])
	case "recover-operator":
		// Handled by cmd/auditor before Run is called.
		fmt.Fprintln(os.Stderr, "recover-operator deve ser despachado pelo binário principal")
		return ExitUsage
	default:
		fmt.Fprintf(os.Stderr, "comando desconhecido %q\n\n", args[0])
		WriteHelp(os.Stderr)
		return ExitUsage
	}
}

// IsCLICommand reports whether the first argument is a CLI subcommand
// (as opposed to starting the HTTP API with no args).
func IsCLICommand(arg string) bool {
	switch arg {
	case "help", "-h", "--help", "migration-risk", "log-anomaly", "dq-contracts", "restore-verify", "recover-operator":
		return true
	default:
		return false
	}
}
