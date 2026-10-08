# Verificação do backlog BA-01 a BA-25

Esta página registra o que foi observado no desenvolvimento que complementa o PR #123. Os IDs BA são do backlog do Notion. Uma implementação aprovada pela CI ainda precisa das verificações de operação e uso real indicadas abaixo.

## Verificações executadas

- A suíte Go passou com PostgreSQL 18 e MongoDB 8 descartáveis. Inclui coleta MongoDB sem leitura de documentos, isolamento de regras PostgreSQL, recuperação de operador, autenticação, exportação paginada, medição de consultas e validação de saneamento por coleta posterior.
- O inventário com 10 mil objetos mostrou 20 linhas por página no navegador. A troca para a página seguinte levou cerca de 322 ms no ambiente local, incluindo a automação do navegador. No servidor, o p95 observado para essa massa foi de aproximadamente 6,1 ms. Esses números descrevem apenas esta máquina e esta massa sintética.
- Na navegação por teclado do inventário, Enter abriu o detalhe e Escape o fechou; a URL voltou ao inventário sem o caminho do objeto. Os nomes dos objetos são botões identificados e focáveis. Uma inspeção programática de contraste dos textos visíveis nesta tela não encontrou razão inferior a 4,5:1.
- PDFs executivo, técnico e por tabela foram renderizados com cenários sem dados, parciais e com texto longo; as primeiras e últimas páginas foram examinadas visualmente. Os limites de tamanho e páginas têm testes automatizados.
- O frontend passou pela geração de tipos OpenAPI, formatação, tipagem, 36 testes e build. `go vet` e `git diff --check` passaram.

## Verificações que dependem do ambiente de aceite

- **BA-01 e BA-06:** medir memória e tempo de primeira interação em navegador representativo, com base pequena e grande; percorrer dashboard, inventário, achados, histórico e relatórios com leitor de tela e teclado. A API de memória não ficou disponível no navegador local usado nesta revisão. A inspeção de contraste de uma tela não substitui auditoria de acessibilidade completa.
- **BA-03, BA-10 e BA-16:** validar restauração de backup e arquivamento com volume e política de retenção da organização. Confirmar criptografia do volume e dos backups em produção, permissões de acesso e descarte dos artefatos. O aplicativo não criptografa PDFs separadamente e não remove histórico de auditoria automaticamente.
- **BA-08 e BA-15:** calibrar limites de login e limiares de alerta com tráfego e coletas reais. Os testes cobrem a lógica, mas dados sintéticos não demonstram taxa de falsos alertas em produção.
- **BA-13, BA-21 a BA-24:** confirmar hipóteses, custos de manutenção, inatividade de contas, retenção e qualidade da amostra com os responsáveis pelo banco analisado. A ausência de sessão nas coletas não prova último acesso; amostras por páginas não têm margem de erro calculável. O auditor apenas sugere ações, nunca as executa no alvo.

## Procedimento de aceite

1. Em homologação, faça backup restaurável do banco de controle e execute as migrações. Confira o início da API, as capacidades PostgreSQL, TimescaleDB e MongoDB e a ausência de regras inaplicáveis.
2. Cadastre alvos com credenciais de leitura mínima e teste uma coleta parcial, uma vazia e uma completa. Confira permissões negadas e mensagens de erro na interface.
3. Execute as verificações humanas e operacionais acima. Registre ferramenta, navegador, tamanho da massa, tempos, uso de memória e resultado por tela no backlog.
4. Revise o PDF com os três formatos e a política de redação. Antes de marcar uma ação como validada, registre a execução externa e faça uma nova coleta comparável.

Não marque um item como integralmente aceito apenas porque seu código foi mesclado. O backlog deve distinguir implementação entregue de aceite operacional pendente.
