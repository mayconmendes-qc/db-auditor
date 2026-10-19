package cli

import (
	"fmt"
	"io"
	"strings"
)

// KnownCommands lists CLI subcommands shown by `auditor help`.
// recover-operator stays registered for operators; API mode runs with no args.
var KnownCommands = []string{
	"help",
	"migration-risk",
	"log-anomaly",
	"recover-operator",
}

// WriteHelp prints the top-level CLI help.
func WriteHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Uso: auditor <comando> [opções]

Comandos:
  help                 Mostra esta ajuda
  migration-risk       Analisa risco de migrations PostgreSQL (MR-01…MR-11)
  log-anomaly          Anomalias em logs JSON (postgres-jsonlog / genérico)
  recover-operator     Redefine a senha de um operador (interativo)

Sem argumentos o binário sobe a API HTTP do auditor.

Segredos (token, senha, DSN) só por variável de ambiente ou arquivo *_FILE.
Flags como --token/--password/--dsn são recusadas.

Códigos de saída:
  0  nada acima do limiar
  1  bloqueio pelo gate de severidade
  2  uso, configuração ou credencial inválida
  3  dados insuficientes, contexto parcial ou erro de parse
  4  recusa de segurança`)
}

// IsHelpArg reports whether args request help.
func IsHelpArg(args []string) bool {
	if len(args) == 0 {
		return true
	}
	a := strings.ToLower(args[0])
	return a == "help" || a == "-h" || a == "--help"
}
