# Timescale Auditor

Serviço read-only de inventário, comparação e diagnóstico dos ambientes TimescaleDB (Tiger Cloud e Datacenter Unifique).

## Stack

- **Backend:** Go 1.27.x, `net/http`, `pgx`/`pgxpool`, `sqlc`, PostgreSQL interno
- **Frontend:** React + TypeScript strict + Tailwind CSS + Vitest + BiomeJS
- **Runtime local:** Podman Compose (`api` + `postgres`)

## Ambiente local

1. Copie `.env.example` para `.env` e defina uma senha local (`POSTGRES_PASSWORD`).
2. Execute:

```bash
podman compose up -d --build
```

3. Verifique:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

O PostgreSQL interno permanece na rede privada do compose e **não publica porta externa** por padrão. O volume `snapshot-store` é persistente.

Encerrar:

```bash
podman compose down
```

Para reiniciar os dados locais, remova o volume explicitamente após o `down`.

### Frontend (desenvolvimento)

```bash
cd web
npm install
npm run dev
```

O frontend consome **apenas** a API HTTP do auditor (nunca PostgreSQL/TimescaleDB diretamente).

## Qualidade

### Backend

```bash
make fmt
make check          # go vet
make staticcheck
make lint           # golangci-lint
make test
```

### Frontend

```bash
cd web
npm run check       # Biome
npm run typecheck
npm run test        # Vitest
```

Ou, a partir da raiz:

```bash
make web-check
make web-test
make web-typecheck
```

## Arquitetura

Estrutura oficial (Sprint 0):

```text
cmd/auditor/          # bootstrap / composition root
internal/
  api/                # handlers HTTP finos
  config/             # env + sanitização de secrets
  database/           # pgxpool + sqlc (quando gerado)
sql/queries/          # SQL tipado (sqlc)
migrations/           # SQL versionado do Snapshot Store
web/                  # React + Tailwind
compose.yaml
Containerfile
```

Collectors, analyzers, comparison e scheduler entram nas sprints seguintes. Novas camadas só devem ser criadas com responsabilidade concreta.

## Segurança

- Ambientes auditados: **somente leitura**.
- Credenciais reais ficam fora do repositório (`.env` no `.gitignore`).
- Connection strings são sanitizadas antes de qualquer log.
- `statement_timeout`, `lock_timeout` e `application_name` são configuráveis.

## Documentação de produto

Especificação funcional, backlog e fluxo de desenvolvimento ficam no Notion (workspace *Anotações / Timescale Auditor*).
