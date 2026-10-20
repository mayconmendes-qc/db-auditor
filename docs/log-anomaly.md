# Anomalias em logs (`log-anomaly`)

Lê logs JSON por linha (arquivo, diretório ou stdin), mascara partes variáveis,
agrupa em templates com id estável (estilo Drain), conta por janela, calcula a
linha de base e aponta anomalias. **Processamento local** no v1 (ADR-0001 D10):
templates, contagens e anomalias **não** sobem à API. A **única exceção** é o
agregado de último login por role (só nome da role e horário mais recente),
enviado opcionalmente com `--submit-last-login` (plano 10 L5 / plano 07 L6).

## CLI

```bash
# PostgreSQL 15+ jsonlog
auditor log-anomaly --path ./pgjson.log --profile postgres-jsonlog --format md

# JSON genérico
auditor log-anomaly --path ./app.jsonl --profile generico \
  --time-field ts --level-field severity --message-field msg --format json

# Stdin
cat postgresql-*.json | auditor log-anomaly --profile postgres-jsonlog

# Último login local (só postgres-jsonlog; só role + horário)
auditor log-anomaly --path ./pgjson.log --last-login-out ultimo-login.json

# Envio do agregado ao ambiente (escopo last-login:write)
export AUDITOR_API_URL=https://auditor.example
export AUDITOR_CLI_TOKEN=dba_…   # ou AUDITOR_CLI_TOKEN_FILE
auditor log-anomaly --path ./pgjson.log \
  --last-login-out ultimo-login.json \
  --submit-last-login --environment <uuid>
```

Segredos só por variável ou `*_FILE`. Flags `--token` / `--password` / `--dsn` são recusadas.

Sem `--submit-last-login` a CLI **não** abre conexão de rede nem de banco (RNF-01).
Com a flag, a **única** chamada de rede é
`POST /api/v1/environments/{id}/account-last-logins` com corpo fechado
(`schema_version` + `entries` de `role`/`last_login_at`). Templates, contagens,
IP, banco e aplicação **não** saem da máquina. Envio rejeitado (credencial, escopo,
tamanho, outro ambiente) → código **2**; a saída local (Markdown/JSON e
`--last-login-out`) é preservada.

### Códigos de saída

| Código | Significado |
|---|---|
| 0 | nada acima do limiar (`--fail-on`, padrão `high`) |
| 1 | bloqueio pelo gate (ex.: pico de erros com severidade alta) |
| 2 | uso, arquivo inexistente, perfil sem horário, `--last-login-out`/`--submit-last-login` em perfil genérico, falha de envio (401/403/413) |
| 3 | >10% linhas inválidas, ou base insuficiente para picos |
| 4 | recusa de segurança (não usado neste modo local) |

### Opções principais

| Flag | Padrão | Notas |
|---|---|---|
| `--profile` | `postgres-jsonlog` | ou `generico` |
| `--path` | (stdin) | arquivo, vários `--path`, ou diretório com `.json`/`.jsonl`/`.log` |
| `--time-field` / `--level-field` / `--message-field` | — | obrigatórios (exceto level) no perfil `generico` |
| `--window` | `5m` | janela de contagem |
| `--baseline-windows` | `12` | K janelas na base |
| `--fail-on` | `high` | gate de severidade |
| `--format` | `md` | `md` ou `json` (`schema_version`) |
| `--last-login-out` | — | arquivo local; `-` inclui a seção na saída |
| `--submit-last-login` | off | POST do agregado; exige `--environment`, `AUDITOR_API_URL`, `AUDITOR_CLI_TOKEN`/`_FILE` |
| `--environment` | — | UUID do ambiente (só com submit) |
| `--api-url` | `AUDITOR_API_URL` | base da API |

## Perfis

### `postgres-jsonlog` (PG 15+)

Campos: `timestamp`, `error_severity`, `message`. Eventos
`connection authorized` alimentam o agregado de último login (campo `user` ou
`user=` na mensagem). IP, banco e aplicação **não** entram no arquivo nem no POST.

### `generico`

Mapeamento explícito de horário, nível e mensagem. O mesmo mascarador se aplica.
`--last-login-out` e `--submit-last-login` são recusados (código 2).

## Limites

- Linha > 64 KB: descartada e contada.
- Máximo 10 000 templates; excedente em `outros` com aviso.
- Memória pensada para streaming (orçamento típico 256 MB).
- Throughput alvo ≥ 50 MB/s em leitura.
- Corpo do POST de último login limitado no servidor (512 KiB; até 5000 roles).

## LGPD / mascaramento (ADR-0001 D16)

Parte de `internal/fingerprint/literals.go` e acrescenta IPs, e-mails, UUIDs,
hashes, caminhos e parâmetros `user=`/`database=`. Saídas (terminal, Markdown,
JSON, arquivo de último login, corpo do POST) **nunca** emitem linhas brutas nem
exemplos. Template sem parte fixa vira `não classificável` e não é exibido.

## Último login (local + envio)

Formato do arquivo e do corpo enviado (`schema_version: "1"`):

```json
{
  "schema_version": "1",
  "entries": [
    { "role": "app_readonly", "last_login_at": "2026-10-19T14:30:00Z" }
  ]
}
```

Só `role` e `last_login_at`. Sem IP, banco ou aplicação. Campo extra no POST →
HTTP 400. Token de outro ambiente ou sem `last-login:write` → HTTP 403 (CA-06).
A revisão de acessos (plano 07) exibe o horário com fonte `log` ao lado de
“sem sessão observada (amostra)”.

## Anomalias (valores iniciais, calibrar)

| Tipo | Condição | Severidade |
|---|---|---|
| Pico | z ≥ 3 e contagem ≥ 20 | baixa; média se erro |
| Pico forte | fator ≥ 8× e ≥ 50 | média; alta se erro/FATAL |
| Template novo | ≥ 10 sem histórico | baixa; média se erro |
| Pico de erros | volume erro ≥ 5× | alta |
| Template sumiu | ≥50% das janelas da base e ausente nas 3 últimas | info |
| Base insuficiente | < 6 janelas | só templates novos; código 3 |
