# Sprint 20 — segurança, operação e retenção

## Primeiro acesso na tela de login

Usuário e senha ficam no `.env`:

```bash
AUDITOR_BOOTSTRAP_USER=admin
AUDITOR_BOOTSTRAP_PASSWORD=db-auditor-local-1
```

A senha do exemplo tem 18 caracteres. O mínimo é 16. O Compose local entrega o `.env` ao container da API. Entre em http://localhost:5173 com `admin` / `db-auditor-local-1`, depois troque a senha do `.env` **antes** da primeira subida se o serviço for compartilhado. Mudar o `.env` depois não atualiza o hash.

A conta `operator` nasce uma vez, quando `auditor_user` está vazia. Se o login falhar:

- as duas variáveis não estavam no `.env` na subida que criou a conta;
- a senha tem menos de 16 caracteres;
- já existe um usuário e o bootstrap foi ignorado. Use uma conta de operador existente para criar outra conta pela API administrativa. Se todos os acessos de operador forem perdidos, preserve os dados e planeje uma recuperação controlada a partir de backup. Não apague `auditor_user`.

`AUDITOR_BOOTSTRAP_PASSWORD_FILE` continua válido apenas quando a variável de senha está vazia. Não coloque a senha no frontend nem no Git além do exemplo local. `deploy/compose.prod.yaml` repassa as variáveis de bootstrap configuradas no ambiente do Compose.

## Migração sem perda de dados

Faça backup do snapshot store antes de atualizar uma instalação que já tem dados.
O schema novo de uma instalação vazia é só `backend/migrations/01_baseline.sql`
e `02_seed_demo.sql`. Os arquivos `03`–`10` foram incorporados no baseline e
removidos. O entrypoint só roda em volume vazio. `make reset-volume` **apaga**
snapshots, findings, baselines e relatórios.

Num volume antigo, a API cria `auditor_user`, `auditor_session`,
`auditor_login_attempt` e `auditor_operation_log` na subida se ainda não existirem. Não aplique o baseline
de novo por cima desse volume.

O `operator` pode criar outras contas pela rota administrativa
`POST /api/v1/auth/users` com `username`, `password` (mínimo 16 caracteres),
`role` (`viewer`, `auditor`, `operator`) e `environments` (lista de UUIDs).
Não-operadores precisam de ambientes explicitamente atribuídos. Sessões são
temporárias e revogáveis. A interface usa cookie HttpOnly, SameSite=Lax e
Secure quando a requisição chega por HTTPS; JavaScript não lê o token. O
`GET /api/v1/auth/me` restaura a identidade e um token CSRF mantido somente
na memória da página. Mutações com sessão por cookie exigem `X-CSRF-Token`.
Clientes de API que já usam bearer continuam aceitos; o login sem
`?mode=cookie` mantém esse contrato. Não armazene o bearer no navegador.
Somente HTTPS deve ser exposto publicamente; a API não termina TLS por conta
própria, então use um proxy HTTPS confiável.

### Login após reload e limite de requisições

O reload consulta `GET /api/v1/auth/me` com o cookie da sessão.
Se a API responder 401, a sessão expirou ou foi revogada e é preciso entrar
novamente. Se responder 429 ou estiver indisponível, a tela oferece uma nova
tentativa sem revogar a sessão; aguarde antes de tentar de novo.

A API limita o login a 120 tentativas por minuto por endereço remoto e 10 por
conta em cinco minutos, usando o banco interno para coordenar réplicas. Os
contadores de conta são limpos após login bem-sucedido. Requisições autenticadas
mantêm limite independente de 600 por minuto por usuário. Para investigar um 429,
veja no painel Network do navegador qual rota respondeu 429 e quantas vezes
ela foi chamada no minuto. O Inventário oferece 20, 50 ou 100 linhas por
página e faz paginação no servidor. Se vários navegadores ou integrações usam a mesma conta, as chamadas
deles também compartilham o limite desse usuário. Não aumente o limite antes
de identificar a origem das chamadas.

Cada destino auditado exige `sslmode=verify-full` e host exato em
`AUDITOR_TARGET_ALLOWED_HOSTS`. Certificados devem encadear para uma CA
confiável no container (ou ser fornecidos por configuração segura do driver).
`AUDITOR_ALLOW_INSECURE_LOCAL_TARGETS=true` é uma exceção **apenas para
localhost/loopback em desenvolvimento** com `sslmode=disable`.

## Credenciais e rotação

Hoje as credenciais de bancos auditados continuam em secrets/variáveis do
backend e nunca são enviadas ao navegador. Para girá-las: crie credencial
somente leitura nova no alvo, atualize o secret fora do Git, reinicie a API,
confirme conexão e coleta, então revogue a antiga. Não registre DSNs completos.

Armazenamento cifrado no snapshot store é um plano, não está habilitado:
usar envelope encryption (chave de dados aleatória AES-256-GCM por segredo,
nonce exclusivo, chave mestra em KMS/secret manager), guardar `key_version`,
ciphertext e metadados não sensíveis; ler versão antiga e nova durante
rotação, recriptografar em lotes auditados, revogar versão antiga só após
verificação e rollback. Nunca guardar a chave mestra junto com o ciphertext.
Uma futura migração exigirá threat model, testes de recuperação/rotação e
aprovação operacional antes de trocar as variáveis atuais.

## Retenção e minimização

| Dado | Política |
| --- | --- |
| Sessões | Expiram após 8 horas; logout revoga imediatamente. A API remove registros expirados a cada hora. |
| Artefatos PDF | Expiram em 30 dias; o worker remove os expirados. |
| Jobs de relatório | Metadados podem ser mantidos para trilha; definir janela de arquivamento conforme compliance local. |
| Log de operação | Meta de 365 dias; exportar para armazenamento imutável antes de purgar. Purga ainda exige job e aprovação de retenção. |
| Snapshots/runs/findings/eventos | Não apagar automaticamente: baselines, comparações e trilhas dependem deles. Definir janela e exportação/arquivamento após avaliação legal/operacional. |

`AUDITOR_REPORT_REDACT_METADATA` aceita `environment`, `run_id`,
`requested_by`, `service_version`, `rule_version` (separados por vírgula),
ocultando esses campos do cabeçalho de PDFs futuros sem alterar dados
persistidos. Valide os relatórios gerados antes de compartilhar externamente;
evidências e nomes de objetos podem continuar sensíveis.

## Limites operacionais

`AUDITOR_MAX_CONCURRENT_RUNS`, `AUDITOR_MAX_COLLECTOR_WORKERS` e
`AUDITOR_MAX_DATABASE_CONNECTIONS` limitam runs, collectors por run e
conexões por database no processo. `AUDITOR_TARGET_STATEMENT_TIMEOUT` limita
cada consulta no servidor auditado; os collectors opcionais têm limite de
linhas, duração (`AUDITOR_OPTIONAL_COLLECTOR_TIMEOUT`) e custo estimado do
plano (`AUDITOR_OPTIONAL_MAX_PLAN_COST`). Um plano acima do limite vira falha
parcial auditável, não execução irrestrita. O inventário usa páginas de até 100 linhas
na interface e até 500 por requisição na API. Inspecione planos `EXPLAIN (ANALYZE, BUFFERS)` em cópia de
dados representativa antes de elevar limites. Os índices de paginação ficam no
baseline.

O Inventário não oferece “Todas”: os totais vêm do servidor e os objetos são
carregados por página, inclusive hypertables e agregados contínuos.
