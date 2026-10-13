import type { NavigationSection } from "../../types";

export interface DocTerm {
  name: string;
  meaning: string;
}

export interface DocTopic {
  id: string;
  title: string;
  intro: string;
  steps: string[];
  notes: string[];
  terms: DocTerm[];
  goTo?: NavigationSection;
}

export const docTopics: DocTopic[] = [
  {
    id: "comecar",
    title: "Por onde começar",
    intro:
      "O DB Auditor mostra um retrato do banco: o que existe, o que merece atenção e o que mudou desde a última visita. Ele só observa. Não corrige, não apaga dados e não muda permissões no banco analisado. Um aviso é um convite para a equipe confirmar, não uma ordem para agir.",
    steps: [
      "Entre com o usuário e a senha que a pessoa administradora passou para você.",
      "No seletor de ambiente, escolha o banco que quer ver. Se ele não aparecer, peça acesso. Quem consulta um ambiente não vê os outros.",
      "Se ainda não houver coleta, abra Execuções e inicie uma auditoria, se o seu papel permitir.",
      "Antes de concluir que está tudo bem, leia a cobertura. Coleta parcial ou com falha significa que parte da informação não chegou.",
      "Em um computador compartilhado, use Sair. A sessão dura até oito horas. Recarregar a página não pede a senha de novo enquanto ela valer.",
    ],
    notes: [
      "Quem só consulta é viewer. Quem registra decisões e pede relatórios é auditor. Quem administra contas é operator.",
      "O endereço e a senha do banco analisado ficam no servidor. Eles não aparecem na tela nem no relatório.",
      "O link da página guarda a seção e o ambiente. Enviar esse endereço abre o mesmo lugar para quem tiver acesso.",
    ],
    terms: [
      {
        name: "Ambiente",
        meaning:
          "Um banco, ou um grupo de bancos, que o auditor acompanha. Cada ambiente tem um nome na tela.",
      },
      {
        name: "Coleta",
        meaning:
          "Uma visita somente leitura. O auditor anota a estrutura e as estatísticas que o banco deixou ver naquele momento.",
      },
      {
        name: "Cobertura",
        meaning:
          "O quanto da visita deu certo. Sem cobertura suficiente, uma lista vazia não significa ausência de problema.",
      },
    ],
  },
  {
    id: "dashboard",
    title: "Dashboard",
    intro:
      "O painel é o resumo do ambiente escolhido: espaço, achados, nota e as cinco prioridades do momento. Use-o para decidir por onde começar, não para fechar um diagnóstico.",
    steps: [
      "Escolha o ambiente na barra lateral. A opção de ver todos soma os ambientes que você pode acessar.",
      "Leia o aviso do topo. Ele diz se a conexão está boa, se a coleta foi parcial ou se falta permissão.",
      "Os cartões levam à tela correspondente: achados, armazenamento ou execuções.",
      "No gráfico, cada barra é uma execução identificável. Um intervalo sem coleta não é tratado como zero.",
      "A nota só aparece quando a coleta e a análise necessárias terminaram. Compare notas apenas entre visitas compatíveis.",
    ],
    notes: [
      "A nota é uma média ponderada. Achados reduzem a nota. Chaves primárias verificadas ajudam a parte de estrutura. Os pesos aparecem no próprio painel.",
      "Uma mudança de categoria é uma pista, não a prova da causa.",
      "As cinco prioridades são os sinais mais urgentes da última análise concluída. Abra cada um para ler a evidência.",
    ],
    terms: [
      {
        name: "Nota",
        meaning:
          "Um número de apoio, com versão e pesos visíveis. Ela fica indisponível quando falta coletor ou a análise não terminou.",
      },
      {
        name: "Prioridade",
        meaning:
          "Um achado destacado para a leitura executiva. Severidade alta não garante ganho se a equipe aplicar a sugestão.",
      },
    ],
    goTo: "Dashboard",
  },
  {
    id: "ambientes",
    title: "Ambientes",
    intro:
      "Aqui ficam os bancos que o auditor conhece. A tela mostra nome, tipo e se a conexão está disponível. Credenciais não são editadas neste lugar.",
    steps: [
      "Confira se o ambiente que você espera está na lista.",
      "Veja o tipo: PostgreSQL, TimescaleDB ou MongoDB. O tipo muda o que a coleta consegue observar.",
      "Se o status estiver indisponível, a visita não vai conseguir ler esse banco. Peça a quem opera o servidor para revisar o acesso de leitura.",
      "Depois de uma mudança de conexão no servidor, a API precisa ser reiniciada. Em seguida, confira Status e o painel.",
    ],
    notes: [
      "Criar ou apontar um ambiente novo é trabalho de quem administra o servidor, não um formulário desta tela.",
      "Dois ambientes de demonstração podem existir numa instalação nova. Eles só ganham dados quando a conexão é preenchida e uma coleta termina.",
    ],
    terms: [
      {
        name: "Somente leitura",
        meaning:
          "A conta usada na coleta pode consultar metadados. Ela não recebe ordem de criar, apagar ou alterar objetos.",
      },
      {
        name: "Descoberta",
        meaning:
          "A forma de achar um banco ou vários bancos no servidor já configurado.",
      },
    ],
    goTo: "Ambientes",
  },
  {
    id: "execucoes",
    title: "Execuções",
    intro:
      "Uma execução é uma visita completa: o auditor percorre os coletores, grava o que viu e marca o resultado. Inventário, achados e relatórios apontam para uma execução.",
    steps: [
      "Escolha o ambiente e inicie a auditoria, se o seu papel permitir. O pedido pede confirmação.",
      "Escolha o perfil de coleta. Um perfil mais amplo olha mais coisas e pode demorar mais.",
      "Acompanhe o progresso de cada coletor. Enquanto houver visita em andamento, a lista se atualiza sozinha.",
      "Abra a execução para ver erros, avisos e o que foi de fato coletado.",
      "Uma visita em andamento pode ser cancelada por um operator. O cancelamento é cooperativo: o que já foi gravado permanece.",
    ],
    notes: [
      "Sucesso: a visita terminou. Ainda assim, confira a cobertura dos coletores.",
      "Sucesso parcial: uma parte não pôde ser lida. Lista vazia, nesse caso, pode ser falta de dados.",
      "Falha: leia a mensagem. Em geral é conexão ou permissão da conta de leitura.",
      "Sem coleta ainda, não interprete inventário, nota ou tendência.",
    ],
    terms: [
      {
        name: "Coletor",
        meaning:
          "Uma parte da visita. Um lê tabelas, outro lê índices, outro lê permissões, e assim por diante.",
      },
      {
        name: "Perfil",
        meaning:
          "O conjunto de coletores desta visita. Comparar visitas de perfis diferentes pede cuidado.",
      },
      {
        name: "Baseline",
        meaning:
          "Uma execução aprovada como referência. Aprovar outra exige confirmação e cobertura suficiente.",
      },
    ],
    goTo: "Execuções",
  },
  {
    id: "inventario",
    title: "Inventário",
    intro:
      "O inventário é a lista do que existia na execução escolhida: tabelas, índices, visões, funções e, no TimescaleDB, tabelas temporais e agregados. No MongoDB, a lista mostra coleções e índices. O auditor não lê os documentos.",
    steps: [
      "Escolha o ambiente. Sem ambiente, a lista não tem de onde vir.",
      "Filtre por banco, esquema e tipo de objeto. A busca reduz a lista pelo nome.",
      "Use as páginas para percorrer o resultado. Ordenar uma coluna vale para o resultado inteiro, não só para a página que está na tela.",
      "Abra uma linha para ver o detalhe daquele objeto. Feche pelo botão, pelo fundo ou com Esc. O endereço volta para a lista.",
      "Copie o link do painel quando quiser mostrar exatamente aquele objeto para outra pessoa com acesso.",
    ],
    notes: [
      "Cada painel separa visão geral, estrutura, relacionamentos, performance, segurança e recomendações. Se um dado não veio, o texto diz se foi não coletado, não aplicável, vazio ou sem permissão.",
      "O número de linhas é uma estimativa do PostgreSQL. Ele pode diferir da contagem real.",
      "A definição SQL de visões, índices e funções fica protegida, porque pode conter valores sensíveis. Uma impressão digital ajuda a perceber que algo mudou sem mostrar o texto.",
      "Índice com zero varreduras não deve ser removido só por isso. Confira há quanto tempo as estatísticas foram observadas e se a carga habitual passou por ali.",
      "No MongoDB, verificações feitas para PostgreSQL aparecem como não aplicáveis.",
    ],
    terms: [
      {
        name: "Tabela temporal",
        meaning:
          "Uma tabela do Timescale organizada no tempo, ou em outra dimensão, e dividida em pedaços.",
      },
      {
        name: "Agregado contínuo",
        meaning:
          "Um resumo que o Timescale mantém atualizado a partir de uma origem. Sem política visível, ele ainda pode ser atualizado à mão.",
      },
      {
        name: "Não coletado",
        meaning:
          "O campo não veio nesta visita. Isso não prova que o recurso está desligado nem que o objeto está seguro.",
      },
    ],
    goTo: "Inventário",
  },
  {
    id: "mapeamentos",
    title: "Mapeamentos",
    intro:
      "Mapeamento é dizer que um objeto de um ambiente corresponde a um objeto de outro. Sem esse par, a comparação de estrutura fica sem referência.",
    steps: [
      "Tenha pelo menos dois ambientes com inventário já coletado.",
      "Peça a sugestão automática. Ela propõe pares parecidos. Nada é validado sozinho.",
      "Revise cada sugestão. Confirme o par certo e recuse o que não for o mesmo objeto.",
      "Se a sugestão não achar um par, associe à mão quando você souber que os dois lados são o mesmo objeto.",
    ],
    notes: [
      "Nome igual não garante que seja o mesmo objeto. Olhe o tipo e a estrutura antes de validar.",
      "Um par recusado continua no histórico. Ele não é apagado em silêncio.",
    ],
    terms: [
      {
        name: "Impressão digital",
        meaning:
          "Uma assinatura do objeto, feita com nome, tipo e estrutura, usada para sugerir pares.",
      },
      {
        name: "Par validado",
        meaning:
          "Uma correspondência que uma pessoa confirmou. A comparação de desvio usa esses pares.",
      },
    ],
    goTo: "Mapeamentos",
  },
  {
    id: "desvio",
    title: "Desvio de schema",
    intro:
      "Desvio de schema é a diferença de estrutura entre dois ambientes: o que está igual, o que mudou e o que existe só de um lado. A tela mostra a diferença. Ela não aplica correção.",
    steps: [
      "Escolha a origem e o destino. Os dois precisam ter inventário comparável.",
      "Leia as categorias: igual, diferente, só na origem e só no destino.",
      "Abra um item para ver o que divergiu, como coluna, tipo ou índice.",
      "Use o resultado como evidência para a equipe do banco. A mudança, se houver, acontece fora do auditor.",
    ],
    notes: [
      "Versão, perfil de coleta e cobertura mudam o que dá para comparar. Uma ausência pode ser objeto que não entrou na visita.",
      "Ambientes de mecanismos diferentes, como PostgreSQL e MongoDB, não têm a mesma estrutura para comparar.",
    ],
    terms: [
      {
        name: "Igual",
        meaning:
          "Os dois lados parecem o mesmo objeto, sem diferença relevante de estrutura nesta comparação.",
      },
      {
        name: "Diferente",
        meaning:
          "O par existe nos dois lados, mas alguma parte da estrutura não coincide.",
      },
      {
        name: "Só de um lado",
        meaning: "O objeto apareceu na origem ou no destino, e não no outro.",
      },
    ],
    goTo: "Desvio de schema",
  },
  {
    id: "comparar",
    title: "Comparar servidores",
    intro:
      "Esta tela coloca lado a lado o último retrato de dois ambientes. Serve para ver diferenças de configuração e de tamanho sem abrir uma sessão nova no banco.",
    steps: [
      "Escolha o ambiente da esquerda e o da direita. Eles precisam ser diferentes.",
      "Leia o relatório gerado a partir dos snapshots já gravados.",
      "Trate diferença de parâmetro, como memória compartilhada, como informação. Ela não é, sozinha, um defeito.",
      "Se o retrato de um lado for antigo ou parcial, repita a coleta antes de concluir que os servidores divergiram.",
    ],
    notes: [
      "A comparação não liga de novo no banco auditado. Ela usa o que a última visita guardou.",
      "Para diferença objeto a objeto, use Desvio de schema. Esta tela é o panorama dos dois servidores.",
    ],
    terms: [
      {
        name: "Snapshot",
        meaning:
          "A cópia do que foi observado numa execução. Comparar servidores usa o snapshot mais recente de cada lado.",
      },
    ],
    goTo: "Comparar",
  },
  {
    id: "achados",
    title: "Achados",
    intro:
      "Um achado é um sinal de uma regra: algo que vale investigar. A tela explica o que foi visto, em que objeto, com que confiança e o que a equipe pode fazer em seguida. O auditor não executa a correção.",
    steps: [
      "Filtre por gravidade, estado e tema. Desempenho e Segurança são filtros desta mesma lista, não páginas separadas.",
      "Abra o achado. Leia a constatação, a evidência, a cobertura e a orientação.",
      "Se o seu papel permitir, reconheça o sinal, planeje uma ação ou justifique uma supressão.",
      "Para comparar duas visitas, escolha as duas execuções no painel de diferenças. A lista mostra o que surgiu, sumiu ou mudou.",
      "Exporte CSV ou JSON quando precisar levar a lista para fora. Não inclua senha nem dado de linha nesse arquivo.",
    ],
    notes: [
      "Gravidade é uma prioridade inicial, não um ganho garantido.",
      "Confiança baixa ou coleta parcial pede uma confirmação a mais.",
      "A consulta de confirmação é um modelo. Troque os parâmetros, revise o texto e rode só num ambiente autorizado.",
      "Suprimir não apaga o histórico. O sinal pode ficar quieto por um período. Se voltar depois disso, ele reaparece.",
      "Uma visita incompleta não serve para dizer que um achado foi resolvido.",
    ],
    terms: [
      {
        name: "Regra",
        meaning:
          "Uma verificação automática. Ela produz o achado. Uma pessoa decide o que fazer com ele.",
      },
      {
        name: "Gravidade",
        meaning:
          "Crítica, alta, média, baixa ou informativa. Ajuda a ordenar a fila, não a provar impacto.",
      },
      {
        name: "Supressão",
        meaning:
          "Um registro de que o sinal não deve chamar atenção agora, com motivo e prazo.",
      },
    ],
    goTo: "Findings",
  },
  {
    id: "performance",
    title: "Performance",
    intro:
      "Performance reúne os sinais de espaço, manutenção, índices e consultas. Na página Análises, use o filtro Desempenho. Abra o item: o painel explica a melhoria e o passo a passo.",
    steps: [
      "Escolha o ambiente e a execução que quer ler.",
      "Abra cada sinal e confira se a evidência cobre uma janela representativa da carga.",
      "Cruze com o Inventário: tamanho de tabela, tamanho de índice e histórico do objeto.",
      "Planeje a manutenção com a equipe do banco. Nada desta tela roda sozinho no banco analisado.",
    ],
    notes: [
      "Um índice sem uso na amostra pode ser necessário num relatório mensal que não passou pela coleta.",
      "Estatísticas reiniciadas fazem contadores voltarem a zero. O aviso de reinício importa antes de comparar.",
      "Tendência de crescimento só é estimada quando as duas coletas comparadas terminaram por completo.",
    ],
    terms: [
      {
        name: "Manutenção",
        meaning:
          "Rotinas como limpeza e atualização de estatísticas. O auditor aponta atraso ou custo. A execução fica com o administrador do banco.",
      },
      {
        name: "Consulta agrupada",
        meaning:
          "Consultas parecidas, reunidas sem os valores literais, para ver o padrão e não cada chamada isolada.",
      },
    ],
    goTo: "Findings",
  },
  {
    id: "seguranca",
    title: "Segurança",
    intro:
      "Segurança reúne os sinais de permissão, papéis e funções privilegiadas. Na página Análises, use o filtro Segurança. A ausência de um alerta só tranquiliza quando a coleta daquela verificação terminou.",
    steps: [
      "Comece pela gravidade crítica e alta.",
      "Abra o achado e veja o objeto, a permissão e o que foi de fato observado.",
      "Contas sem sessão na amostra, ou com validade vencida, pedem confirmação com a equipe. A amostra não prova que a conta ficou sem uso entre as visitas.",
      "Não revogue acesso a partir da tela. Combine a mudança e um plano de retorno fora do auditor.",
    ],
    notes: [
      "Função marcada como SECURITY DEFINER executa com os privilégios de quem é dono dela. Isso pede revisão humana. A marca sozinha não é uma falha comprovada.",
      "Permissões de índice, nesta coleta, são as da tabela dona.",
      "O corpo das funções não é exibido.",
    ],
    terms: [
      {
        name: "Permissão efetiva",
        meaning:
          "O que a coleta conseguiu ver sobre quem pode usar aquele objeto, não a lista completa de tudo que a pessoa pode fazer no servidor.",
      },
      {
        name: "Evidência",
        meaning:
          "O fato observado: objeto, permissão, configuração ou contagem. Sem evidência, o texto diz o que faltou.",
      },
    ],
    goTo: "Findings",
  },
  {
    id: "regras",
    title: "Regras",
    intro:
      "Regras são as verificações que geram as análises. Em Análises, o botão Regras do ambiente abre o catálogo: a versão e se a regra está ligada no esquema escolhido. Uma regra que não se aplica ao tipo de banco não é um resultado saudável. Ela simplesmente não roda ali.",
    steps: [
      "Escolha o ambiente e o esquema.",
      "Leia o nome da regra e a explicação em português: o que ela procura e o que não afirma.",
      "Veja se a regra está ativa para aquele esquema. Quem opera pode ajustar a exceção permitida. A mudança fica registrada.",
      "Para ver os sinais já produzidos, volte para Achados e filtre pela regra.",
    ],
    notes: [
      "O texto da regra descreve constatação, evidência, impacto e próximo passo. O impacto só fala em número quando há métrica e denominador.",
      "Atualizar o catálogo muda a versão usada na análise. Relatórios guardam a versão com a qual foram gerados.",
    ],
    terms: [
      {
        name: "Catálogo",
        meaning:
          "A lista versionada de verificações. A mesma versão deve ser usada para comparar duas análises.",
      },
      {
        name: "Não aplicável",
        meaning:
          "A regra não faz sentido naquele mecanismo, por exemplo uma verificação de PostgreSQL num catálogo MongoDB.",
      },
    ],
    goTo: "Findings",
  },
  {
    id: "relatorios",
    title: "Relatórios",
    intro:
      "O relatório em PDF é uma leitura para compartilhar: resumo executivo, detalhe técnico ou uma tabela. Ele é pedido em segundo plano e só nasce de uma execução concluída, com a análise das regras também concluída.",
    steps: [
      "Escolha o ambiente, a execução concluída e o tipo: executivo, técnico ou tabela.",
      "No tipo tabela, informe banco, esquema e tabela.",
      "Acompanhe a fila. Você pode cancelar um pedido ainda em andamento ou tentar de novo quando a tela permitir.",
      "Baixe o PDF enquanto ele existir. Confira a cobertura no documento antes de usar os números.",
      "Se o arquivo estiver grande, ele pode vir truncado. O próprio PDF diz o que foi cortado.",
    ],
    notes: [
      "O PDF fica guardado por 30 dias. Depois disso o arquivo some, mas o registro de que ele foi pedido permanece.",
      "Há um código do arquivo para conferir se o download é o mesmo que foi gerado.",
      "O servidor pode ocultar alguns identificadores no cabeçalho, se essa opção estiver ligada.",
      "Uma execução parcial não é usada para afirmar que um achado foi resolvido.",
    ],
    terms: [
      {
        name: "Fila",
        meaning:
          "A lista de pedidos. O PDF não sai no instante do clique. A tela avisa quando ele fica pronto.",
      },
      {
        name: "Redação",
        meaning:
          "Uma opção de esconder identificadores no PDF, para compartilhar o conteúdo com menos contexto interno.",
      },
    ],
    goTo: "Relatórios",
  },
  {
    id: "acompanhamento",
    title: "Acompanhamento",
    intro:
      "Acompanhamento é o caderno da equipe: baseline aprovado, anotações de implantação ou manutenção, e alertas de regressão. Um alerta só nasce de duas coletas completas, para não gritar por causa de uma visita pela metade.",
    steps: [
      "Escolha o ambiente.",
      "Veja qual execução está aprovada como referência.",
      "Registre uma implantação ou uma manutenção, com uma nota curta do que aconteceu fora do auditor.",
      "Leia os alertas de regressão. Cada um mostra a referência e a evidência.",
      "Reconheça um alerta quando a equipe já souber do que se trata. O reconhecimento pede um motivo.",
    ],
    notes: [
      "Aprovar baseline é uma decisão. Ela pede cobertura suficiente.",
      "Avisos de execução, como falha de coleta, continuam aparecendo mesmo quando a visita foi parcial. O que fica mais quieto é o alerta de regressão de achado.",
      "Anote o que mudou no mundo real. Sem isso, a próxima comparação não tem contexto.",
    ],
    terms: [
      {
        name: "Regressão",
        meaning:
          "Um sinal de que algo piorou em relação à referência, nas duas visitas completas comparadas.",
      },
      {
        name: "Anotação",
        meaning:
          "Um registro humano de implantação ou manutenção, preso ao ambiente e à data.",
      },
    ],
    goTo: "Acompanhamento",
  },
  {
    id: "acoes",
    title: "Ações assistidas",
    intro:
      "Uma ação assistida é um plano para a equipe: o que conferir, o risco, como validar e como voltar atrás. O auditor escreve o plano. A mudança, se acontecer, é feita fora dele, no banco, por quem tem essa responsabilidade.",
    steps: [
      "Escolha um achado e registre o responsável.",
      "Leia os passos de confirmação antes de qualquer mudança.",
      "Depois que a mudança for feita lá fora, marque a ação como executada externamente.",
      "Rode uma nova coleta comparável e veja a medição. Uma diferença observada não prova, sozinha, que a mudança foi a causa.",
      "Exporte o histórico em JSONL ou CSV quando precisar da trilha completa do ambiente. Confira a marca de fim do arquivo.",
    ],
    notes: [
      "O diagnóstico opcional de qualidade lê uma amostra limitada, com conta de leitura, e guarda só contagens. A margem de erro dessa amostra não é conhecida.",
      "Compare visitas com cobertura, perfil e carga parecidos. Visitas diferentes demais não sustentam a conclusão.",
    ],
    terms: [
      {
        name: "Medição",
        meaning:
          "A comparação cautelosa de um número antes e depois, entre duas visitas parecidas.",
      },
      {
        name: "Amostra",
        meaning:
          "Um pedaço limitado de linhas. Ela pode não representar a tabela inteira.",
      },
    ],
    goTo: "Ações assistidas",
  },
  {
    id: "contas",
    title: "Contas e acesso",
    intro:
      "Cada pessoa entra com a própria conta. A senha não deve ser compartilhada. Quem administra as contas é o operator. Os outros papéis só veem e agem dentro dos ambientes que receberam.",
    steps: [
      "Entre com a sua conta. Se o computador for compartilhado, saia ao terminar.",
      "Viewer consulta. Auditor também registra decisão e pede relatório. Operator administra contas, execuções e baselines.",
      "Para criar uma conta, um operator informa o nome, uma senha longa, o papel e os ambientes permitidos.",
      "O código do aplicativo autenticador é opcional. Em Contas, gere o segredo, confirme o código de seis dígitos e guarde os códigos de recuperação. Só um operator pode passar a exigir esse código na própria conta.",
      "Se perder o acesso de todos os operators, não apague as contas. Peça o procedimento de recuperação a quem opera o servidor.",
    ],
    notes: [
      "Trocar a senha no arquivo de configuração, depois que a conta já existe, não altera a senha gravada.",
      "Não há envio de código por SMS.",
      "Senha de banco e connection string não aparecem nesta tela.",
    ],
    terms: [
      {
        name: "Sessão",
        meaning:
          "O período em que você permanece dentro da aplicação, até oito horas, ou até usar Sair.",
      },
      {
        name: "Escopo",
        meaning:
          "Os ambientes que uma conta pode abrir. Operator vê a administração. As outras contas ficam nos ambientes marcados.",
      },
      {
        name: "Código de recuperação",
        meaning:
          "Uma reserva de uso único, mostrada uma vez, para entrar se o aplicativo autenticador não estiver à mão.",
      },
    ],
    goTo: "Contas",
  },
  {
    id: "status",
    title: "Status",
    intro:
      "Status diz se a aplicação está de pé: a API, o banco interno onde os retratos ficam guardados, as conexões com os ambientes e as tarefas recentes.",
    steps: [
      "Abra Status quando uma tela parecer parada ou vazia sem explicação.",
      "Confira se a API e o armazenamento interno estão saudáveis.",
      "Veja a conexão de cada ambiente. Indisponível ou sem configuração explica uma coleta que não começa.",
      "Se você acabou de mudar a configuração do servidor, reinicie a API e volte aqui antes de iniciar outra visita.",
    ],
    notes: [
      "Métricas internas não ficam publicadas no endereço do site. Elas existem só na rede de operação.",
      "Um ambiente saudável aqui ainda pode ter uma coleta parcial. Os dois avisos contam histórias diferentes.",
    ],
    terms: [
      {
        name: "API",
        meaning:
          "O programa que a tela consulta. Se ela não responde, as páginas não conseguem carregar dados.",
      },
      {
        name: "Banco interno",
        meaning:
          "Onde o auditor guarda contas, execuções, inventário e achados. Não é o banco analisado.",
      },
    ],
    goTo: "Status",
  },
  {
    id: "duvidas",
    title: "Quando algo parecer errado",
    intro:
      "Antes de concluir que o banco piorou, ou que a tela quebrou, confira se você está olhando o lugar certo. A maior parte das surpresas vem de ambiente, execução ou filtro.",
    steps: [
      "Confira o ambiente selecionado, a execução e os filtros.",
      "Veja se a coleta e a análise terminaram e se a cobertura está completa.",
      "Leia o aviso da tela. Ele pode falar de permissão, limite, versão incompatível ou erro temporário.",
      "Repita a ação só se ela for segura. Para uma falha que continua, envie a quem opera o horário, o ambiente, o identificador da execução e o código do pedido, se a mensagem mostrar um.",
      "Não envie senha, connection string nem conteúdo de linha.",
    ],
    notes: [
      "Lista vazia com coleta parcial não é um banco limpo.",
      "Um número que caiu a zero pode ser reinício de estatística, não uma melhora.",
      "Se o guia desta página não abrir um tópico, clique de novo no título. Só um tópico fica aberto por vez. O texto aparece logo abaixo dele.",
    ],
    terms: [
      {
        name: "Filtro",
        meaning:
          "Uma restrição da lista, como gravidade ou esquema. Limpar o filtro mostra de novo o conjunto inteiro daquela execução.",
      },
    ],
  },
];
