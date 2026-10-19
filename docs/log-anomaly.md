# Anomalias em logs (`log-anomaly`)

Lê logs JSON por linha (arquivo, diretório ou stdin), mascara partes variáveis,
agrupa em templates com id estável (estilo Drain), conta por janela, calcula a
linha de base e aponta anomalias. **Processamento local** no v1 (ADR-0001 D10):
templates, contagens e anomalias não sobem à API. A única exceção prevista é o
agregado de último login por role (lote 5, após o plano 07 L6) — este documento
cobre a geração **local** desse insumo.

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
```

Segredos só por variável ou `*_FILE`. Flags `--token` / `--password` / `--dsn` são recusadas.

Não há conexão com banco nem com rede neste modo (RNF-01), salvo o envio opcional
de último login (ainda não ligado — plano 10 L5).

### Códigos de saída

| Código | Significado |
|---|---|
| 0 | nada acima do limiar (`--fail-on`, padrão `high`) |
| 1 | bloqueio pelo gate (ex.: pico de erros com severidade alta) |
| 2 | uso, arquivo inexistente, perfil sem horário, `--last-login-out` em perfil genérico |
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

## Perfis

### `postgres-jsonlog` (PG 15+)

Campos: `timestamp`, `error_severity`, `message`. Eventos
`connection authorized` alimentam o agregado de último login (campo `user` ou
`user=` na mensagem). IP, banco e aplicação **não** entram no arquivo.

### `generico`

Mapeamento explícito de horário, nível e mensagem. O mesmo mascarador se aplica.
`--last-login-out` é recusado (código 2).

## Limites

- Linha > 64 KB: descartada e contada.
- Máximo 10 000 templates; excedente em `outros` com aviso.
- Memória pensada para streaming (orçamento típico 256 MB).
- Throughput alvo ≥ 50 MB/s em leitura.

## LGPD / mascaramento (ADR-0001 D16)

Parte de `internal/fingerprint/literals.go` e acrescenta IPs, e-mails, UUIDs,
hashes, caminhos e parâmetros `user=`/`database=`. Saídas (terminal, Markdown,
JSON, arquivo de último login) **nunca** emitem linhas brutas nem exemplos.
Template sem parte fixa vira `não classificável` e não é exibido.

## Último login (local)

Formato do arquivo (`schema_version: "1"`), o mesmo que o lote 5 enviará:

```json
{
  "schema_version": "1",
  "entries": [
    { "role": "app_readonly", "last_login_at": "2026-10-19T14:30:00Z" }
  ]
}
```

Só `role` e `last_login_at`. Sem IP, banco ou aplicação.

## Anomalias (valores iniciais, calibrar)

| Tipo | Condição | Severidade |
|---|---|---|
| Pico | z ≥ 3 e contagem ≥ 20 | baixa; média se erro |
| Pico forte | fator ≥ 8× e ≥ 50 | média; alta se erro/FATAL |
| Template novo | ≥ 10 sem histórico | baixa; média se erro |
| Pico de erros | volume erro ≥ 5× | alta |
| Template sumiu | ≥50% das janelas da base e ausente nas 3 últimas | info |
| Base insuficiente | < 6 janelas | só templates novos; código 3 |
