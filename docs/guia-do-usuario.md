# Guia do usuário — DB Auditor

O DB Auditor ajuda a conhecer um banco, encontrar sinais de risco e acompanhar melhorias. Ele **não corrige, remove ou altera dados no banco analisado**. As recomendações são hipóteses para a equipe responsável confirmar antes de executar qualquer mudança fora da aplicação.

Hoje os coletores prontos atendem PostgreSQL e TimescaleDB. A tela de capacidades informa o que foi ou não foi coletado. A ausência de um alerta só é tranquilizadora quando a coleta e a análise terminaram com cobertura suficiente.

## 1. Entrar e escolher um ambiente

Abra o endereço fornecido pelo administrador e entre com sua conta. A sessão dura até oito horas. Ao recarregar a página, o aplicativo verifica se ela ainda é válida. Use **Sair** em um computador compartilhado.

Escolha um ambiente no seletor. Cada ambiente representa um banco ou conjunto de bancos configurado pelo administrador. Se ele não aparecer, peça ao operador que confira o acesso da sua conta. Um **viewer** consulta dados; um **auditor** registra decisões e solicita diagnósticos; um **operator** também administra contas e configurações autorizadas.

Os endereços e credenciais dos bancos analisados ficam na configuração do servidor. Eles não são digitados na interface nem aparecem nos relatórios.

## 2. Fazer uma coleta e entender a cobertura

Em **Execuções**, selecione o ambiente e inicie uma auditoria quando seu papel permitir. Escolha o perfil de coleta adequado: perfis mais amplos observam mais aspectos e podem levar mais tempo. A tela mostra a execução, os coletores, avisos e falhas.

- **Sucesso:** a execução terminou; confira ainda a cobertura dos coletores usados na conclusão.
- **Sucesso parcial:** parte dos dados não pôde ser coletada. Uma lista vazia pode significar falta de dados, não ausência de problemas.
- **Falha:** verifique a mensagem e peça ao operador que revise conectividade e permissões da conta de leitura.
- **Sem coleta:** inicie uma execução antes de interpretar inventário, score ou tendências.

O identificador da execução permite relacionar inventário, achados, gráficos e relatórios à mesma observação.

## 3. Navegar pelo inventário

Em **Inventário**, escolha banco, esquema e tipo de objeto. A busca e os filtros reduzem a lista. Use os controles de página para percorrer objetos; o total indica quantos atendem ao filtro. Clique no cabeçalho de uma coluna para ordenar **todo o resultado filtrado**, inclusive as outras páginas. Abrir uma linha mostra detalhes e histórico do objeto; fechar o painel devolve a URL à lista.

Tabelas, índices, visualizações, funções, hypertables e agregados contínuos dependem das capacidades da coleta. Se um tipo não se aplica ao mecanismo ou não foi coletado, a aplicação indica a limitação. Um número de linhas é estimado pelo PostgreSQL e pode diferir da contagem real.

## 4. Investigar achados

Em **Findings**, filtre por severidade, estado e tipo. A **constatação** resume o que foi observado em português. Abra o achado para ver evidência, confiança, cobertura, objeto, orientação e histórico. Os termos e textos originais da regra ficam na seção técnica.

Um achado é um sinal para investigar. Severidade indica prioridade inicial, não ganho garantido. Confiança baixa ou coleta parcial pede confirmação adicional. O auditor pode reconhecer um achado, planejar uma ação, justificar uma supressão e registrar o resultado. Se o sinal reaparecer, o histórico anterior permanece visível.

Antes de usar uma consulta de confirmação, substitua os parâmetros pelo objeto correto, revise o SQL e execute-o somente em um ambiente autorizado. O auditor não executa a correção sugerida.

## 5. Ler o dashboard e acompanhar mudanças

O **Dashboard** mostra indicadores atuais e a evolução por execução: armazenamento, achados, score e cobertura. Escolha período, granularidade e ambiente. Cada barra representa uma execução identificável; intervalos sem coleta não são tratados como zero. Avisos indicam coletas parciais, versões ou perfis incompatíveis e reinício observado de contadores.

O **score** é uma nota ponderada, com versão e pesos mostrados no produto. Ele fica indisponível quando faltam coletores necessários ou a análise não foi concluída. Compare notas somente entre execuções compatíveis. Uma mudança de score é uma pista para investigar, não prova de causa.

Em **Acompanhamento**, um auditor pode aprovar um baseline, registrar manutenção ou implantação e reconhecer alertas de regressão. Um alerta persistente mostra sua referência e a evidência. Não interprete um intervalo sem coleta como melhora.

## 6. Medir uma ação antes e depois

Em **Ações assistidas**, escolha uma ação e duas execuções: a primeira antes da alteração externa e a segunda depois. Informe a hipótese, a janela observada e o que sabe sobre a carga. Marque a confirmação de carga semelhante somente se tiver evidência para isso. A aplicação verifica ainda perfil, versões, análise, objeto e cobertura.

O resultado mostra valores anteriores e posteriores, comparabilidade e ressalvas. Uma diferença observada **não comprova que a ação a causou**. Se um achado persistir após uma medição comparável, a ação volta para análise. O JSON e o CSV de ações incluem o histórico de medições disponível na lista, limitado a 100 medições por achado.

## 7. Diagnóstico opcional de qualidade de dados

Um operador precisa habilitar o recurso e configurar uma conta de leitura sem privilégios de escrita. Em **Ações assistidas**, informe banco, esquema, tabela, limite de até 1.000 linhas e as colunas que deseja examinar. Você pode verificar nulos inesperados, chaves candidatas repetidas, datas fora de um intervalo, distribuição concentrada e referências órfãs em relações simples.

O resultado guarda **contagens**, não valores de linhas. Em tabelas grandes, o PostgreSQL seleciona páginas para a amostra; em tabelas menores, a leitura limitada pode seguir a ordem física. Nenhum dos dois métodos fornece uma margem de erro estatística calculável neste produto. Confirme qualquer hipótese na população de dados antes de uma mudança.

O plano sugerido descreve confirmação, dependências, backup, janela, validação e recuperação. Para dados fora de uma política de retenção, obtenha aprovação da área de negócio, teste a cópia para arquivo e a restauração antes de considerar exclusão externa. Registre a evidência da nova coleta ou da validação humana ao concluir uma ação.

## 8. Gerar e compartilhar um PDF

Em **Relatórios**, escolha ambiente, execução concluída, tipo de relatório e filtros. A geração ocorre em segundo plano; acompanhe o estado e baixe o arquivo quando estiver pronto. O relatório executivo resume riscos e próximos passos. O técnico acrescenta evidências e inventário. O relatório por tabela foca um objeto.

O PDF informa escopo, execução, cobertura, versão das regras, score, gráficos e análise final. Quando existe baseline, destaca novos achados altos ou críticos. Se o conteúdo ultrapassar os limites, o próprio documento indica o recorte ou a geração falha com uma explicação. Os limites atuais são 500 bancos, 5.000 tabelas, 2.000 achados, 200 páginas e 16 MiB.

Antes de compartilhar, escolha a redação apropriada:

- **Sem redação:** inclui identificadores e evidências disponíveis; compartilhe apenas com quem pode acessá-los.
- **Identificadores:** oculta nomes, evidências e textos originais sensíveis, preservando gráficos agregados.
- **Estrita:** também retira listas de objetos, evolução de armazenamento e detalhes do baseline; preserva contagens e a proveniência do pedido.

O artefato expira após 30 dias. A expiração remove o PDF, mas preserva o registro do pedido. Um novo pedido pode gerar outro arquivo. O hash exibido no histórico permite conferir a integridade do artefato enquanto ele existir.

## 9. Outras áreas

- **Performance:** consulte sinais de consultas, índices, manutenção, espaço e carga. Confirme planos e estatísticas em uma janela representativa antes de agir.
- **Segurança:** revise papéis, permissões, funções privilegiadas e exposição observável. Uma revogação depende da equipe da aplicação e de um plano de retorno.
- **Mapeamentos, Desvio de schema e Comparar:** relacione objetos entre ambientes e examine diferenças. Versão, perfil e cobertura podem limitar a comparação.
- **Regras:** veja o catálogo, sua versão e a explicação das verificações. Uma regra não aplicável ao mecanismo não deve ser interpretada como resultado saudável.
- **Status:** veja se API, armazenamento interno, conexões e tarefas estão funcionando.
- **Contas:** operadores gerenciam usuários e acessos. Não compartilhe senha; peça revogação se perder um dispositivo.
- **Documentação:** consulte ajuda contextual e o glossário dentro do aplicativo.

## 10. Se algo parecer errado

1. Confira o ambiente, a execução e os filtros selecionados.
2. Verifique se a coleta e a análise terminaram e se a cobertura é completa.
3. Leia o aviso exibido; ele pode indicar falta de permissão, limite, versão incompatível ou erro temporário.
4. Repita a ação apenas se ela for segura. Para falhas persistentes, envie ao operador o horário, ambiente, identificador da execução e `X-Request-ID` da resposta. Não envie credenciais nem dados de linha.

Para instalar, atualizar e fazer backup, use o [guia de operação](ops.md). Para conhecer limites e decisões técnicas da terceira etapa, use [operações da terceira etapa](third-stage-operations.md).
