# SDD-GUIDE.md — Spec-Driven Development no game-backend

> Baseado no padrão GitHub Spec Kit (2025/2026)
> Referência: github.com/github/spec-kit

---

## O que é SDD

Spec-Driven Development é a metodologia onde a spec é a fonte de verdade, não o código.
O agente de IA implementa a partir de arquivos estruturados, não de instruções vagas em chat.

Princípio central: cada fase produz um artefato markdown que alimenta a próxima.

    spec.md -> plan.md -> tasks.md -> código

Quando o requisito muda: edite a spec primeiro, depois regenere em cascata.
Nunca mude o código diretamente e atualize a spec depois. A spec sempre lidera.

### Escopo: o que NÃO é SDD

SDD aplica-se a **features de comportamento** (os serviços Go). **Infra e deploy** (Terraform, provisionamento de banco, LocalStack, k8s/Agones) **não** usam o ciclo spec→plan→tasks: são tratados como **tasks da própria feature** (o que o serviço possui — ex: schema/migrations) ou como **itens de backlog** (provisionamento cloud, deploy), documentados de forma leve (ex: README junto do Terraform). Não criar specs SDD para infra.

---

## Estrutura de pastas no monorepo

Cada feature é uma pasta numerada dentro de specs/ do seu serviço.

```
services/
├── identity-service/
│   └── specs/
│       ├── 001-account-auth/
│       │   ├── spec.md          <- O QUE construir
│       │   ├── plan.md          <- COMO construir
│       │   └── tasks.md         <- lista de trabalho atômico
│       └── 002-session-jwt/
│           ├── spec.md
│           ├── plan.md
│           └── tasks.md
│
├── achievements-service/
│   └── specs/
│       └── 001-unlock-system/
│           ├── spec.md
│           ├── plan.md
│           ├── tasks.md
│           └── data-model.md    <- opcional: modelo de dados detalhado
│
├── analytics-service/
│   └── specs/
│       └── 001-event-ingestion/
│           ├── spec.md
│           ├── plan.md
│           └── tasks.md
│
└── matchmaking-service/
    └── specs/
        └── 001-queue-streaming/
            ├── spec.md
            ├── plan.md
            ├── tasks.md
            └── contracts/       <- opcional: contratos gRPC detalhados
                └── matchmaking-proto.md
```

### Regras de nomenclatura

| Item             | Formato              | Exemplo            |
|------------------|----------------------|--------------------|
| Pasta da feature | NNN-slug-kebab-case  | 001-account-auth   |
| Spec             | sempre spec.md       | spec.md            |
| Plan             | sempre plan.md       | plan.md            |
| Tasks            | sempre tasks.md      | tasks.md           |
| Extras opcionais | nome descritivo      | data-model.md      |

- Numeração sequencial POR SERVIÇO — cada serviço começa em 001
- Não existe numeração global pro monorepo
- Nunca reutilize um número — feature cancelada deixa o número vago

---

## O que vai em cada arquivo

### spec.md — O QUE o sistema faz

Comportamento e regras de negócio em EARS. Sem decisões técnicas.
Responde: o que o sistema deve fazer?

```markdown
# Spec: NNN-nome-da-feature
**Serviço:** <nome>-service
**Status:** Draft | Approved | Implemented
**Data:** YYYY-MM-DD

## Contexto
[Uma ou duas frases: por que esta feature existe?]

## Comportamento

### <Nome do caso de uso>
- WHEN <evento> -> <resultado>
- IF <condição inválida> -> retorna <erro específico>
- THE SYSTEM SHALL <invariante sempre verdadeiro>

## Contratos de Interface

### <NomeService> (Port de entrada)
- Metodo(ctx, param) -> (retorno, error)

### <NomeRepository> (Port de saída)
- Metodo(ctx, param) -> (retorno, error)

## Modelo de Dados
[Campos, tipos e restrições]

## Regras de Validação
| Campo | Regra | Erro |

## Casos de Teste Obrigatórios
- [ ] <cenário feliz>
- [ ] <cenário de erro>
- [ ] <edge case>

## Fora do escopo
[O que esta feature explicitamente NÃO faz]
```

---

### plan.md — COMO construir

Decisões técnicas derivadas da spec. Arquitetura, libs, trade-offs.
Responde: como vamos implementar?

```markdown
# Plan: NNN-nome-da-feature
**Spec:** specs/NNN-nome-da-feature/spec.md
**Data:** YYYY-MM-DD

## Abordagem técnica
[Resumo das decisões de implementação]

## Arquivos a criar ou modificar
- internal/domain/player.go        — tipos e interfaces
- internal/service/player_service.go — lógica de negócio
- internal/repository/postgres.go  — acesso ao banco
- internal/transport/http_handler.go — handlers HTTP
- migrations/001_create_players.sql

## Compute e banco (ver ADR-0005)
- Compute escolhido para este serviço e por quê
- Banco: gerenciado (DynamoDB/RDS) vs auto-provisionado

## Dependências e libs
- github.com/jackc/pgx/v5          — driver PostgreSQL
- github.com/google/uuid           — geração de IDs

## Decisões técnicas e trade-offs
[Se for decisão arquitetural relevante -> propor ADR. Ver docs/adr/ADR-GUIDE.md]

## Ordem de implementação
[Sequência que minimiza dependências bloqueantes]

## Riscos e pontos de atenção
[O que pode dar errado ou precisar de cuidado especial]
```

---

### tasks.md — Lista de trabalho atômico

Tarefas pequenas o suficiente para o agente executar sem ambiguidade.
O agente marca [x] conforme implementa — funciona como checklist vivo.
Você pode parar e retomar em qualquer ponto sem perder o estado.

```markdown
# Tasks: NNN-nome-da-feature
**Plan:** specs/NNN-nome-da-feature/plan.md
**Data:** YYYY-MM-DD

## Fase 1 — Domain
- [ ] Criar internal/domain/player.go com struct Player e interfaces
- [ ] Criar internal/domain/errors.go com os tipos de erro da spec

## Fase 2 — Service
- [ ] Implementar internal/service/player_service.go
- [ ] Escrever unit tests cobrindo todos os Casos de Teste Obrigatórios da spec

## Fase 3 — Repository
- [ ] Implementar internal/repository/postgres_repository.go
- [ ] Criar migration migrations/001_create_players.sql
- [ ] Escrever integration tests com testcontainers

## Fase 4 — Transport
- [ ] Implementar handlers em internal/transport/http_handler.go
- [ ] Mapear erros de domínio para HTTP status codes
- [ ] Escrever handler tests com httptest

## Fase 5 — Wire-up e verificação
- [ ] Conectar dependências em cmd/server/main.go
- [ ] Confirmar que go test ./... -race passa
- [ ] Confirmar que go build ./... passa
```

---

## Fluxo de trabalho completo

### Sessão 1 — Spec (você + agente em modo planejamento)

Descreva a intenção. O agente faz perguntas para eliminar ambiguidade,
depois gera spec.md. Não implementa nada.

    "Me ajuda a escrever a spec do sistema de conquistas do achievements-service.
    Quero desbloqueio automático por evento, progresso parcial salvo,
    e listagem de conquistas por jogador."

Você valida. Se algo estiver errado, edite antes de avançar.

### Sessão 2 — Plan

    "A spec está aprovada. Gere o plan.md para specs/001-unlock-system/."

Agente gera plan.md com decisões técnicas.
Você valida a abordagem, a ordem, os riscos.

### Sessão 3 — Tasks

    "Plan aprovado. Gere o tasks.md quebrando em tarefas atômicas."

Você confere se as tarefas fazem sentido e se a ordem está correta.

### Sessão 4 — Implementação

    "Tasks aprovadas. Implemente seguindo specs/001-unlock-system/tasks.md.
    Siga a arquitetura hexagonal do CLAUDE.md."

Agente executa task por task, marcando [x] conforme conclui.

### Mudança de requisito pós-implementação

    1. Edite spec.md com a mudança
    2. Peça ao agente para atualizar plan.md se necessário
    3. Peça ao agente para adicionar novas tasks em tasks.md
    4. Peça ao agente para atualizar código e testes

---

## Formato EARS para regras no spec.md

| Padrão           | Quando usar                   | Exemplo                                               |
|------------------|-------------------------------|-------------------------------------------------------|
| WHEN X -> Y      | Evento dispara comportamento  | WHEN jogador completa 10 partidas -> SHALL desbloquear "Veterano" |
| IF X -> Y        | Condição de validação         | IF nome estiver vazio -> retorna ErrInvalidName       |
| THE SYSTEM SHALL | Invariante sempre verdadeiro  | THE SYSTEM SHALL garantir unicidade de nome (ci)      |
| WHILE X, Y       | Comportamento contínuo        | WHILE jogador na fila, SHALL notificar status a cada 30s |

---

## INSTRUÇÕES PARA O AGENTE DE IA

> Esta seção é direcionada ao Claude Code. Aplique em toda sessão neste repositório.

### Antes de qualquer implementação

1. Verifique se existe specs/NNN-<feature>/spec.md para o que está sendo pedido
2. Se não existir -> pergunte ao usuário antes de prosseguir. Nunca invente comportamento
3. Se existir spec mas não houver plan -> ofereça gerar plan.md antes de implementar
4. Se existir plan mas não houver tasks -> ofereça gerar tasks.md antes de implementar
5. Nunca pule fases sem confirmação do usuário

### Ao gerar spec.md

- Use EARS para todas as regras de comportamento
- Seja específico nos erros (nome do tipo, não só "retorna erro")
- Liste todos os casos de teste obrigatórios — viram unit tests
- Preencha "Fora do escopo" para evitar scope creep
- NÃO tome decisões técnicas no spec.md — isso vai no plan.md

### Ao gerar plan.md

- Liste exatamente quais arquivos serão criados ou modificados
- Defina compute e banco do serviço (ADR-0005), justificando a escolha
- Se a decisão técnica for arquiteturalmente significativa -> proponha ADR (docs/adr/ADR-GUIDE.md)
- Documente trade-offs considerados, não só a decisão final

### Ao gerar tasks.md

- Cada task deve ser atômica: uma responsabilidade, um arquivo de destino
- Ordene por dependência: domain antes de service, service antes de transport
- Inclua tasks de teste junto das tasks de implementação (não só no final)
- Use [ ] para tasks pendentes — marque [x] conforme implementar

### Durante a implementação

- Execute task por task na ordem do tasks.md
- Marque [x] em cada task concluída antes de avançar para a próxima
- Se encontrar ambiguidade não coberta pela spec -> pare e pergunte, não chute
- Ao final: rode go test ./... -race e confirme que todos os casos da spec passam

### Nomenclatura de pastas

- Descubra o próximo número inspecionando ls services/<nome>/specs/
- Formato: NNN-slug-kebab-case sem acentos
- Crie a pasta e os arquivos (spec, plan, tasks) na sequência correta

---

## Prompts úteis por fase

Criar spec:
    "Me ajuda a escrever a spec de <feature> para o <serviço>.
    Quero <descrição de intenção>. Use o formato do SDD-GUIDE.md."

Gerar plan a partir de spec aprovada:
    "A spec em specs/001-<feature>/spec.md está aprovada.
    Gere o plan.md com as decisões técnicas de implementação."

Gerar tasks a partir de plan aprovado:
    "Plan aprovado. Gere o tasks.md com tarefas atômicas ordenadas por dependência."

Implementar:
    "Tasks aprovadas. Implemente seguindo specs/001-<feature>/tasks.md.
    Siga a arquitetura hexagonal descrita no CLAUDE.md."

Atualizar após mudança de requisito:
    "O requisito mudou: <descrição da mudança>.
    Atualize specs/001-<feature>/spec.md e mostre o que muda no plan e nas tasks."

Verificar alinhamento spec vs código:
    "Revise o <serviço> e verifique se o código implementa todos os casos
    da spec em specs/001-<feature>/spec.md. Liste divergências encontradas."
