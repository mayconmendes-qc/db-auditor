# DB Auditor

O DB Auditor é um aplicativo que **olha para o seu banco de dados sem alterar nada** e mostra, em português e com telas simples, o que existe, o que merece atenção e o que mudou desde a última visita.

Ele funciona com **PostgreSQL**, **TimescaleDB** e com o catálogo do **MongoDB** (coleções e índices; o auditor não lê o conteúdo dos documentos).

> Em uma frase: o auditor só observa. Ele não apaga dados, não revoga acessos e não executa manutenção no banco analisado. Um aviso é um convite para a equipe investigar — nunca uma ordem automática.

---

## Comece em 5 minutos

Se você só quer ver o sistema funcionando no seu computador, siga estes passos. Não é preciso saber programar.

### O que você precisa ter instalado

Só uma destas opções:

| Opção | Como conferir se já está instalada |
| --- | --- |
| **Podman** + Compose | No terminal: `podman --version` e `podman compose version` |
| **Docker** + Compose | No terminal: `docker --version` e `docker compose version` |

Não é necessário instalar Go, Node, Bun nem PostgreSQL na máquina: o projeto sobe tudo dentro de containers.

**Exemplos de instalação**

- Fedora / RHEL: `sudo dnf install podman podman-compose`
- macOS (Homebrew): `brew install podman` ou instale o [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- Outros sistemas: use o instalador oficial do Podman ou do Docker

Se for usar **Docker**, avise o projeto uma vez (ou grave no ambiente):

```bash
make up COMPOSE="docker compose"
```

### Passo a passo

1. **Baixe o projeto** (se ainda não tiver a pasta no computador):

   ```bash
   git clone https://github.com/osmendes/db-auditor.git
   cd db-auditor
   ```

2. **Crie o arquivo de configuração** a partir do modelo:

   ```bash
   cp .env.example .env
   ```

3. **Abra o arquivo `.env`** em qualquer editor de texto e ajuste pelo menos estes valores:

   | O que é | Variável | O que fazer |
   | --- | --- | --- |
   | Senha do banco interno do auditor | `POSTGRES_PASSWORD` | Troque `change-me-locally` por uma senha sua |
   | Usuário do primeiro login | `AUDITOR_BOOTSTRAP_USER` | O exemplo é `admin` |
   | Senha do primeiro login | `AUDITOR_BOOTSTRAP_PASSWORD` | Precisa ter **no mínimo 16 caracteres**. O exemplo local é `db-auditor-local-1` |
   | Nome da empresa (aparece na tela) | `AUDITOR_COMPANY_NAME` | Ex.: `Minha Empresa` |

   Para só experimentar, os valores de exemplo do `.env.example` já funcionam em máquina local. Troque as senhas antes de expor o serviço na internet.

4. **Suba o sistema**:

   ```bash
   make up
   ```

   Na primeira vez pode demorar alguns minutos (baixa imagens e monta os containers). Quando terminar, o terminal mostra os endereços da API e das telas.

5. **Abra no navegador**: [http://localhost:5173](http://localhost:5173)

   Entre com o usuário e a senha do passo 3.

6. **(Opcional)** Confira se a API está saudável:

   ```bash
   make smoke
   ```

   Se aparecer `smoke ok`, a parte de servidor respondeu bem.

### Comandos úteis do dia a dia

| Quero… | Comando |
| --- | --- |
| Parar tudo | `make down` |
| Ver o que está acontecendo | `make logs` |
| Ver se os containers estão no ar | `make ps` |
| Recomeçar do zero em desenvolvimento | `make reset-volume` (apaga dados internos — **não use** onde já há coletas importantes) |

---

## Depois do login: o que fazer

1. Escolha um **ambiente** no seletor (cada ambiente é um banco configurado).
2. Em **Execuções**, inicie uma auditoria (se o seu papel permitir).
3. Aguarde a coleta terminar e confira se a cobertura foi completa, parcial ou se falhou.
4. Explore o **Dashboard**, o **Inventário** e os **Achados**.
5. Quando quiser um resumo para compartilhar, use **Relatórios** (PDF).

Guia completo das telas: [docs/guia-do-usuario.md](docs/guia-do-usuario.md).

---

## Qual problema ele resolve

Bancos crescem: tabelas, índices, permissões e rotinas se acumulam. Com o tempo fica difícil responder:

- O que existe neste ambiente e quanto espaço ocupa?
- Há sinais de risco (permissão ampla, índice sem uso, rotina privilegiada)?
- Isso piorou ou melhorou desde a última coleta?
- Dá para mostrar um resumo para quem não lê SQL?

O DB Auditor faz uma **coleta somente leitura**, guarda o retrato e mostra o resultado em telas: painel, inventário, achados, comparação entre execuções e relatório em PDF.

---

## O que você consegue fazer

- Entrar com uma conta e escolher o ambiente.
- Rodar uma auditoria e ver se a coleta foi completa, parcial ou falhou.
- Percorrer tabelas, índices, visões, funções e, no TimescaleDB, tabelas temporais e agregados.
- No MongoDB, ver coleções e índices (sem ler documentos).
- Ler achados com o que foi observado, a evidência e um próximo passo sugerido.
- Comparar duas execuções, acompanhar regressões e baixar um PDF.
- Opcionalmente, exigir código do autenticador (TOTP) para contas de operador.

Os endereços e senhas dos bancos analisados ficam na configuração do servidor. Eles **não** são digitados na tela e **não** aparecem nos relatórios.

---

## Ligar um banco para ser auditado

Ainda no arquivo `.env`, preencha os ambientes que a tela vai mostrar. Use uma conta **somente leitura** no banco de destino.

```bash
AUDITOR_COMPANY_NAME=Nome da empresa

AUDITOR_TARGET_1_NAME=Banco principal
AUDITOR_TARGET_1_ENVIRONMENT_ID=00000000-0000-0000-0000-000000000001
AUDITOR_TARGET_1_TYPE=self_hosted
AUDITOR_TARGET_1_DISCOVERY=single_database
AUDITOR_TARGET_1_HOST=endereco-do-banco
AUDITOR_TARGET_1_PORT=5432
AUDITOR_TARGET_1_DATABASE=nome_do_banco
AUDITOR_TARGET_1_USER=usuario_somente_leitura
AUDITOR_TARGET_1_PASSWORD=senha-desse-usuario
AUDITOR_TARGET_1_SSLMODE=verify-full
```

O segundo espaço segue o mesmo padrão (`AUDITOR_TARGET_2_...`), até o espaço 20. Se nenhum id for preenchido, uma instalação nova cria dois ambientes de demonstração.

**Importante**

- Informe o endereço em `AUDITOR_TARGET_ALLOWED_HOSTS`. Sem isso, o auditor recusa a conexão.
- O padrão de TLS é `verify-full` (confere o certificado).
- Depois de salvar o `.env`: `make down && make up`.
- Um volume que já existe não apaga ambientes antigos. O mesmo id com nome novo só atualiza o nome; um id novo cria outro ambiente.
- Não rode `make reset-volume` numa instalação que já tem coletas: isso apaga o banco interno (snapshots, achados e relatórios).

Para publicar em um servidor de verdade (domínio, HTTPS, backup), use o [guia de operação](docs/ops.md) e o exemplo em `deploy/env.prod.example`.

---

## Endereços neste computador

| O quê | Onde abrir |
| --- | --- |
| Telas (interface) | [http://localhost:5173](http://localhost:5173) |
| API | [http://localhost:8080](http://localhost:8080) |
| Saúde da API | [http://localhost:8080/api/v1/status](http://localhost:8080/api/v1/status) |

A tela fala com a API pelo mesmo endereço do navegador (isso mantém o login no cookie).

> **Atenção — porta 9090 (métricas):** a API expõe métricas em `:9090` **sem autenticação**. O Compose **não** publica essa porta no seu computador. Não acrescente `9090:9090` em `ports:` e não abra essa porta no firewall. Detalhes no [guia de operação](docs/ops.md).

---

## Se algo der errado

| Sintoma | O que tentar |
| --- | --- |
| Porta já em uso (`address already in use`) | No `.env`, mude `FRONTEND_HTTP_PORT` (ex.: `5174`) ou `AUDITOR_HTTP_PORT` e rode `make up` de novo |
| Página não abre | Confira `make ps` e `make logs`. Espere a primeira subida terminar |
| Login não funciona | Confira `AUDITOR_BOOTSTRAP_USER` e `AUDITOR_BOOTSTRAP_PASSWORD` no `.env`. A senha precisa ter 16+ caracteres. Mudar o `.env` **depois** da primeira criação **não** troca a senha já gravada |
| Ambiente não aparece | Peça ao operador para conferir o acesso da sua conta, ou confira se os `AUDITOR_TARGET_*` foram preenchidos antes da primeira subida |
| Coleta falha | Verifique conectividade, usuário somente leitura, `AUDITOR_TARGET_ALLOWED_HOSTS` e TLS |
| Perdeu todas as contas de operador | Não apague usuários. Siga a recuperação no [guia de operação](docs/ops.md#recuperar-o-acesso-de-operador) |

---

## Pastas do projeto

```text
.
├── backend/       programa que coleta, analisa e guarda os resultados
├── frontend/      telas do navegador
├── deploy/        publicação em servidor (proxy, HTTPS, backup)
├── docs/          guias para usar e operar
├── compose.yaml   sobe banco interno, API e telas juntos
├── Makefile       atalhos: subir, parar, testar
├── .env.example   modelo de configuração (copie para .env)
└── README.md      este texto
```

Textos principais:

- [Guia do usuário](docs/guia-do-usuario.md) — dia a dia nas telas
- [Guia de operação](docs/ops.md) — instalar, backup, produção

---

## Tecnologias

| Peça | Para quê |
| --- | --- |
| Go | API e regras de análise |
| PostgreSQL | banco interno (contas, coletas, achados) |
| MongoDB (driver) | leitura do catálogo, quando o ambiente é MongoDB |
| React, TypeScript e Vite | telas |
| Tailwind CSS | visual das telas |
| Bun | instalar e verificar o frontend |
| Podman ou Docker Compose | subir tudo sem instalar as linguagens na máquina |
| Caddy | HTTPS na instalação de produção |

---

## Como contribuir

Toda ajuda é bem-vinda: texto mais claro, correção, teste ou função nova.

1. Crie um ramo a partir de `main` (ex.: `docs/ajuste-do-guia`).
2. Faça uma mudança pequena e fácil de revisar.
3. **Nunca** envie o arquivo `.env`, senha, chave ou connection string. Use só o `.env.example` com valores falsos.
4. Se a mudança mexe em comportamento, rode:

   ```bash
   make backend-fmt backend-check backend-test
   make frontend-check frontend-typecheck frontend-test
   ```

5. Abra um Pull Request e diga o que mudou e por quê.

Não é obrigatório conhecer toda a arquitetura para melhorar documentação ou um texto da tela. Em dúvida de produto, comece pelo [guia do usuário](docs/guia-do-usuario.md).

---

## Cuidados que valem para todo mundo

- O banco analisado é só de leitura. Recomendações não são executadas sozinhas.
- Segredos ficam fora do Git.
- Senhas e endereços de conexão não devem aparecer em logs.
- Listas de bancos e schemas podem limitar o que entra na coleta.
- Um PDF pedido fica guardado por 30 dias; o registro do pedido permanece.
