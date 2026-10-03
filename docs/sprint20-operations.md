# Sprint 20 — segurança, operação e retenção

## Primeiro acesso na tela de login

Não existe senha embutida. O usuário é o valor de `AUDITOR_BOOTSTRAP_USER`
(no `.env.example`, `admin`). A senha é o conteúdo de
`secrets/bootstrap-password`, com no mínimo 16 caracteres, sem aspas e sem
espaço no fim. O Compose local publica esse arquivo em
`/run/secrets/bootstrap-password`.

A conta é criada uma única vez, como `operator`, quando `auditor_user` está
vazia. Entre em http://localhost:5173 com esse par. Se o login falhar:

- a API não subiu porque o arquivo ou a variável não existiam no primeiro boot;
- a senha tem menos de 16 caracteres;
- o arquivo foi alterado depois da criação da conta (isso não atualiza o hash);
- já havia outro usuário e o bootstrap foi ignorado.

Depois do primeiro login bem-sucedido, retire o arquivo do host ou substitua
por um segredo gerenciado. Não coloque a senha no frontend nem no Git.
`deploy/compose.prod.yaml` ainda não monta esse segredo: não use esse compose
como está para o primeiro acesso.

## Migração sem perda de dados

Faça backup do snapshot store antes da atualização. Em um volume PostgreSQL existente,
aplique **somente** `backend/migrations/10_sprint20_identity.sql` com uma conta de
administração e confirme as tabelas `auditor_user`, `auditor_session` e
`auditor_operation_log` antes de iniciar a nova API. O diretório
`docker-entrypoint-initdb.d` só roda em volumes vazios. `make reset-volume`
**apaga** snapshots, findings, baselines e relatórios e não é um passo de migração.

O procedimento do arquivo `secrets/bootstrap-password` está na seção
[Primeiro acesso na tela de login](#primeiro-acesso-na-tela-de-login).

O `operator` pode criar outras contas pela rota administrativa
`POST /api/v1/auth/users` com `username`, `password` (mínimo 16 caracteres),
`role` (`viewer`, `auditor`, `operator`) e `environments` (lista de UUIDs).
Não-operadores precisam de ambientes explicitamente atribuídos. Sessões são
temporárias e revogáveis; o frontend as guarda somente na memória da aba.
Somente HTTPS deve ser exposto publicamente; a API não termina TLS por conta
própria, então use um proxy HTTPS confiável.

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
parcial auditável, não execução irrestrita. O inventário usa páginas de até 500 linhas
por requisição; a opção Todas busca lotes e deve ser usada com filtros em
ambientes grandes. Inspecione planos `EXPLAIN (ANALYZE, BUFFERS)` em cópia de
dados representativa antes de elevar limites; índices da migration 10 cobrem
ordenação estável das listas e o snapshot mais recente.

A opção Todas interrompe a operação e apresenta erro se ultrapassar 500.000
linhas; ela nunca exibe um subconjunto como se fosse o resultado completo.
Refine os filtros para conjuntos maiores.
