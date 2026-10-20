package cli

import (
	"fmt"
	"os"
	"strings"
)

// SecretFlagNames are CLI flags that must never carry secrets as arguments.
var SecretFlagNames = []string{"--token", "--password", "--dsn", "--secret"}

// RefuseSecretFlags returns ExitUsage when a secret-like flag appears in args.
// The message never echoes the flag value.
func RefuseSecretFlags(args []string) error {
	for _, arg := range args {
		name, _, cut := strings.Cut(arg, "=")
		if !cut {
			name = arg
		}
		for _, banned := range SecretFlagNames {
			if name == banned {
				return fmt.Errorf("recuse passar segredos em argumentos (%s); use a variável de ambiente ou o arquivo *_FILE correspondente", banned)
			}
		}
	}
	return nil
}

// ReadSecret loads a secret from env, or from the path in envFile when set.
// Prefer *_FILE when both are present so operators can mount secrets as files.
func ReadSecret(env, envFile string) (string, error) {
	if path := strings.TrimSpace(os.Getenv(envFile)); path != "" {
		data, err := os.ReadFile(path) //nolint:gosec // G703: path is operator-configured *_FILE, not a request parameter.
		if err != nil {
			return "", fmt.Errorf("não foi possível ler %s", envFile)
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	}
	return strings.TrimRight(os.Getenv(env), "\r\n"), nil
}

// RequireCLIToken returns AUDITOR_CLI_TOKEN / AUDITOR_CLI_TOKEN_FILE or ExitUsage.
func RequireCLIToken() (string, error) {
	token, err := ReadSecret("AUDITOR_CLI_TOKEN", "AUDITOR_CLI_TOKEN_FILE")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("defina AUDITOR_CLI_TOKEN ou AUDITOR_CLI_TOKEN_FILE")
	}
	return token, nil
}
