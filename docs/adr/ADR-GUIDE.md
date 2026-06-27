# ADR-GUIDE.md — Architecture Decision Records no game-backend

> Guia de ADRs para o projeto e instruções para o agente de IA criar ADRs automaticamente
> Baseado em: Michael Nygard (formato original), AWS Prescriptive Guidance, Martin Fowler

---

## O que é um ADR

Um ADR é um documento curto que captura uma única decisão arquitetural. A ênfase está em *único* e *curto*: um ADR, uma decisão. "Usar PostgreSQL no player-service" é um ADR. "Definir toda a estratégia de banco de dados" é um design doc, não um ADR.

ADRs contêm não apenas a decisão, mas também uma breve justificativa. Devem resumir o problema que levou à decisão e os trade-offs considerados. O ato de escrevê-los ajuda a clarificar o pensamento — especialmente em grupo.

---

## Por que usar ADRs neste projeto

- O código mostra **o quê** foi construído. O ADR explica **por quê**.
- Cada microsserviço pode evoluir de forma independente — sem ADRs, decisões tomadas num serviço se perdem quando o contexto muda.
- O agente de IA começa cada sessão sem memória. ADRs são o histórico de raciocínio que ele precisa para não contradizer decisões anteriores.
- Quando uma decisão mudar (e vai mudar), o ADR documenta a evolução — não apaga o passado.

---

## Quando criar um ADR

### Criar SEMPRE que houver:

| Categoria | Exemplos |
|---|---|
| **Estrutura** | Monorepo vs polyrepo, separação de serviços, uso de módulos Go |
| **Padrão arquitetural** | Hexagonal arch, event-driven, CQRS, saga pattern |
| **Tecnologia** | Escolha de framework, banco de dados, message broker, lib principal |
| **Infraestrutura** | ECS vs Lambda, DynamoDB vs RDS, Terraform vs CDK |
| **Protocolo de comunicação** | REST vs gRPC, síncrono vs assíncrono, SQS vs Kafka |
| **Segurança** | Estratégia de autenticação, modelo de autorização |
| **Observabilidade** | Stack de métricas, tracing, logging estruturado |
| **CI/CD** | Estratégia de deploy, branching model, pipeline structure |
| **Trade-off explícito** | Qualquer decisão onde alternativas foram descartadas |

### NÃO criar ADR para:

- Decisões de implementação interna (nome de variável, estrutura de loop)
- Convenções já capturadas no `CLAUDE.md` (estilo de código, ferramentas de lint)
- Decisões reversíveis de baixo impacto sem alternativas consideradas
- Tarefas operacionais do dia a dia

---

## Formato do ADR (template padrão do projeto)

```markdown
# ADR-NNNN: [Título curto e descritivo da decisão]

**Status:** [Proposed | Accepted | Superseded | Deprecated]
**Data:** YYYY-MM-DD
**Serviço(s) afetado(s):** [todos | player-service | matchmaking-service | infra | ...]
**Supersede:** [ADR-XXXX se aplicável]
**Supersedido por:** [ADR-XXXX se aplicável]

---

## Contexto

[Descreve o problema ou situação que motivou esta decisão.
O que está acontecendo? Quais restrições existem? Por que uma decisão é necessária agora?
Seja factual — não defenda a decisão ainda, só descreva o cenário.]

## Alternativas Consideradas

### Opção A: [Nome da opção]
- **Prós:** ...
- **Contras:** ...

### Opção B: [Nome da opção]
- **Prós:** ...
- **Contras:** ...

### Opção C: [Nome da opção — se houver]
- **Prós:** ...
- **Contras:** ...

## Decisão

[Enuncia a decisão de forma clara e assertiva.
"Decidimos usar X porque Y. Descartamos A por causa de B e C por causa de D."]

## Consequências

### Positivas
- [O que fica mais fácil com esta decisão]

### Negativas / Trade-offs
- [O que fica mais difícil ou limitado — seja honesto]

### Neutras / Implicações
- [Outras decisões que serão necessárias como consequência desta]

## Referências
- [Links para docs, benchmarks, discussões relevantes]
```

---

## Numeração e localização dos arquivos

```
docs/adr/
├── 0000-template.md            ← template de referência (não é um ADR real)
├── 0001-monorepo-structure.md
├── 0002-hexagonal-architecture.md
├── 0003-grpc-matchmaking.md
├── 0004-terraform-localstack.md
└── README.md                   ← índice de todos os ADRs
```

Regras de numeração:
- Zero-padded com 4 dígitos: `0001`, `0002`, etc.
- Sequencial e monotônico — nunca reutilizar um número
- Nome do arquivo: `NNNN-slug-da-decisao.md` (kebab-case, sem acentos)
- O ADR funciona como um log append-only. Não edite ADRs aceitos. Se a decisão mudar, escreva um novo ADR que supersede o original e vincule os dois.

---

## Índice de ADRs (`docs/adr/README.md`)

Manter sempre atualizado:

```markdown
# Architecture Decision Records

| # | Título | Status | Data | Serviço |
|---|---|---|---|---|
| [0001](0001-monorepo-structure.md) | Monorepo com módulos Go independentes | Accepted | 2025-XX-XX | todos |
| [0002](0002-hexagonal-architecture.md) | Hexagonal Architecture em todos os serviços | Accepted | 2025-XX-XX | todos |
| [0003](0003-grpc-matchmaking.md) | gRPC com streaming para matchmaking | Accepted | 2025-XX-XX | matchmaking |
```

---

## Ciclo de vida de um ADR

```
Proposed → Accepted → [Superseded | Deprecated]
```

- **Proposed:** escrito, aguardando revisão/validação
- **Accepted:** decisão ativa, em vigor no projeto
- **Superseded:** substituído por ADR mais recente (manter o link)
- **Deprecated:** decisão abandonada sem substituto direto

---

## INSTRUÇÕES PARA O AGENTE DE IA

> Esta seção é direcionada ao Claude Code e outros agentes.
> Leia e aplique durante toda sessão de trabalho neste repositório.

### Detecção automática de momentos ADR

Durante qualquer sessão de trabalho, monitore os seguintes sinais e **proponha criar um ADR** quando identificá-los, sem esperar pedido explícito do usuário:

**Sinais linguísticos do usuário:**
- "vamos usar X em vez de Y"
- "prefiro X porque..."
- "descartamos X, vamos com Y"
- "optei por X"
- "melhor usar X aqui"
- qualquer comparação explícita entre tecnologias ou abordagens

**Sinais de implementação:**
- Você acabou de escolher entre dois frameworks, libs ou serviços AWS
- Você definiu um padrão arquitetural (hexagonal, CQRS, event-driven, etc.)
- Você escolheu um protocolo de comunicação (REST vs gRPC vs SQS vs WebSocket)
- Você tomou uma decisão de infraestrutura (ECS vs Lambda, DynamoDB vs RDS)
- Você estabeleceu uma convenção que afeta mais de um serviço
- Você descartou uma abordagem por razões específicas

**Sinais de mudança:**
- Uma decisão anterior (documentada ou não) está sendo revisitada
- O usuário menciona que "antes usava X, agora vai usar Y"
- Uma spec muda de forma que implica mudança arquitetural

### Fluxo de criação de ADR pelo agente

Quando detectar um momento ADR:

1. **Sinalize** de forma concisa, sem interromper o fluxo:
   ```
   → Detectei uma decisão arquitetural aqui (gRPC para matchmaking vs REST).
     Posso registrar um ADR? [s/n ou "depois"]
   ```

2. **Se aprovado**, preencha o template com:
   - Contexto baseado na conversa atual
   - Alternativas que foram discutidas (ou que você sabe que existiam)
   - A decisão exatamente como foi tomada
   - Consequências honestas — incluindo trade-offs negativos

3. **Determine o próximo número** inspecionando `docs/adr/` e incrementando o maior existente

4. **Apresente o rascunho** para revisão antes de escrever no disco
   ```
   Rascunho do ADR-0003:
   [conteúdo]
   
   Gravar em docs/adr/0003-grpc-matchmaking.md? [s/n]
   ```

5. **Só grave o arquivo após confirmação explícita** do usuário

6. **Atualize `docs/adr/README.md`** adicionando a nova entrada na tabela

### Comportamento proativo vs invasivo

**Proativo (correto):**
- Sinalizar no momento certo, de forma curta, sem bloquear o trabalho
- Propor uma vez e aceitar "não" ou "depois" sem insistir
- Redigir o ADR com qualidade, não só preencher o template mecanicamente

**Invasivo (evitar):**
- Propor ADR para cada linha de código ou decisão trivial
- Bloquear o fluxo de implementação esperando resposta
- Criar ADRs sem aprovação do usuário
- Perguntar mais de uma vez se o usuário disse "depois"

### Quando NÃO propor ADR

- Decisões de implementação interna (nome de função, estrutura de for loop)
- Seguir convenções já documentadas no `CLAUDE.md` ou em ADRs existentes
- Mudanças cosméticas ou refactoring sem impacto arquitetural
- Quando o usuário já sinalizou que não quer ADR para aquela sessão

### Verificação de consistência com ADRs existentes

Antes de implementar qualquer coisa de natureza arquitetural, verifique `docs/adr/README.md`:
- A decisão que você está prestes a implementar conflita com algum ADR aceito?
- Se sim, avise o usuário **antes** de implementar e pergunte se quer superseder o ADR existente

```
⚠️ Isso conflita com ADR-0002 (Hexagonal Architecture).
   A implementação que você pediu coloca lógica de negócio em transport/.
   Quer que eu ajuste a implementação para seguir o ADR, ou prefere supersedê-lo?
```

---

## ADRs iniciais do projeto (a criar)

Os seguintes ADRs devem ser criados no início do projeto para documentar as decisões já tomadas:

| # | Decisão | Contexto |
|---|---|---|
| 0001 | Monorepo com módulos Go independentes | Por que não polyrepo; por que não módulo único |
| 0002 | Hexagonal Architecture em todos os serviços | Por que não MVC clássico; trade-offs de complexidade inicial |
| 0003 | gRPC com streaming para matchmaking | Por que não REST; por que não WebSocket |
| 0004 | Terraform + LocalStack (local) + AWS (prod) | Por que não CDK; por que não Pulumi |
| 0005 | ECS Fargate em vez de Lambda para serviços stateful | Trade-offs de custo vs complexidade |
| 0006 | Prometheus + Grafana em vez de CloudWatch nativo | Trade-offs de vendor lock-in vs custo |
| 0007 | SDD como metodologia de desenvolvimento | Por que não TDD puro; como specs e testes se relacionam |

---

## Referências

- [Michael Nygard — Documenting Architecture Decisions (2011)](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
- [Martin Fowler — Architecture Decision Record](https://martinfowler.com/bliki/ArchitectureDecisionRecord.html)
- [AWS Prescriptive Guidance — ADR Process](https://docs.aws.amazon.com/prescriptive-guidance/latest/architectural-decision-records/adr-process.html)
- [joelparkerhenderson/architecture-decision-record](https://github.com/joelparkerhenderson/architecture-decision-record)
