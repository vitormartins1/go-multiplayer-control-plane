# SDD-GUIDE.md — Spec-Driven Development no game-backend

> Baseado no padrao GitHub Spec Kit (2025/2026)
> Referencia: github.com/github/spec-kit

---

## O que e SDD

Spec-Driven Development e a metodologia onde a spec e a fonte de verdade, nao o codigo.
O agente de IA implementa a partir de arquivos estruturados, nao de instrucoes vagas em chat.

Principio central: cada fase produz um artefato markdown que alimenta a proxima.

    spec.md -> plan.md -> tasks.md -> codigo

Quando o requisito muda: edite a spec primeiro, depois regenere em cascata.
Nunca mude o codigo diretamente e atualize a spec depois. A spec sempre lidera.

---

## Estrutura de pastas no monorepo

Cada feature e uma pasta numerada dentro de specs/ do seu servico.

```
services/
├── player-service/
│   └── specs/
│       ├── 001-player-crud/
│       │   ├── spec.md          <- O QUE construir
│       │   ├── plan.md          <- COMO construir
│       │   └── tasks.md         <- lista de trabalho atomico
│       └── 002-player-rating/
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
| Pasta da feature | NNN-slug-kebab-case  | 001-player-crud    |
| Spec             | sempre spec.md       | spec.md            |
| Plan             | sempre plan.md       | plan.md            |
| Tasks            | sempre tasks.md      | tasks.md           |
| Extras opcionais | nome descritivo      | data-model.md      |

- Numeracao sequencial POR SERVICO — cada servico comeca em 001
- Nao existe numeracao global pro monorepo
- Nunca reutilize um numero — feature cancelada deixa o numero vago

---

## O que vai em cada arquivo

### spec.md — O QUE o sistema faz

Comportamento e regras de negocio em EARS. Sem decisoes tecnicas.
Responde: o que o sistema deve fazer?

```markdown
# Spec: NNN-nome-da-feature
**Servico:** <nome>-service
**Status:** Draft | Approved | Implemented
**Data:** YYYY-MM-DD

## Contexto
[Uma ou duas frases: por que esta feature existe?]

## Comportamento

### <Nome do caso de uso>
- WHEN <evento> -> <resultado>
- IF <condicao invalida> -> retorna <erro especifico>
- THE SYSTEM SHALL <invariante sempre verdadeiro>

## Contratos de Interface

### <NomeService> (Port de entrada)
- Metodo(ctx, param) -> (retorno, error)

### <NomeRepository> (Port de saida)
- Metodo(ctx, param) -> (retorno, error)

## Modelo de Dados
[Campos, tipos e restricoes]

## Regras de Validacao
| Campo | Regra | Erro |

## Casos de Teste Obrigatorios
- [ ] <cenario feliz>
- [ ] <cenario de erro>
- [ ] <edge case>

## Fora do escopo
[O que esta feature explicitamente NAO faz]
```

---

### plan.md — COMO construir

Decisoes tecnicas derivadas da spec. Arquitetura, libs, trade-offs.
Responde: como vamos implementar?

```markdown
# Plan: NNN-nome-da-feature
**Spec:** specs/NNN-nome-da-feature/spec.md
**Data:** YYYY-MM-DD

## Abordagem tecnica
[Resumo das decisoes de implementacao]

## Arquivos a criar ou modificar
- internal/domain/player.go        — tipos e interfaces
- internal/service/player_service.go — logica de negocio
- internal/repository/postgres.go  — acesso ao banco
- internal/transport/http_handler.go — handlers HTTP
- migrations/001_create_players.sql

## Dependencias e libs
- github.com/jackc/pgx/v5          — driver PostgreSQL
- github.com/google/uuid           — geracao de IDs

## Decisoes tecnicas e trade-offs
[Se for decisao arquitetural relevante -> propor ADR. Ver docs/adr/ADR-GUIDE.md]

## Ordem de implementacao
[Sequencia que minimiza dependencias bloqueantes]

## Riscos e pontos de atencao
[O que pode dar errado ou precisar de cuidado especial]
```

---

### tasks.md — Lista de trabalho atomico

Tarefas pequenas o suficiente para o agente executar sem ambiguidade.
O agente marca [x] conforme implementa — funciona como checklist vivo.
Voce pode parar e retomar em qualquer ponto sem perder o estado.

```markdown
# Tasks: NNN-nome-da-feature
**Plan:** specs/NNN-nome-da-feature/plan.md
**Data:** YYYY-MM-DD

## Fase 1 — Domain
- [ ] Criar internal/domain/player.go com struct Player e interfaces
- [ ] Criar internal/domain/errors.go com os tipos de erro da spec

## Fase 2 — Service
- [ ] Implementar internal/service/player_service.go
- [ ] Escrever unit tests cobrindo todos os Casos de Teste Obrigatorios da spec

## Fase 3 — Repository
- [ ] Implementar internal/repository/postgres_repository.go
- [ ] Criar migration migrations/001_create_players.sql
- [ ] Escrever integration tests com testcontainers

## Fase 4 — Transport
- [ ] Implementar handlers em internal/transport/http_handler.go
- [ ] Mapear erros de dominio para HTTP status codes
- [ ] Escrever handler tests com httptest

## Fase 5 — Wire-up e verificacao
- [ ] Conectar dependencias em cmd/server/main.go
- [ ] Confirmar que go test ./... -race passa
- [ ] Confirmar que go build ./... passa
```

---

## Fluxo de trabalho completo

### Sessao 1 — Spec (voce + agente em modo planejamento)

Descreva a intencao. O agente faz perguntas para eliminar ambiguidade,
depois gera spec.md. Nao implementa nada.

    "Me ajuda a escrever a spec do sistema de conquistas do achievements-service.
    Quero desbloqueio automatico por evento, progresso parcial salvo,
    e listagem de conquistas por jogador."

Voce valida. Se algo estiver errado, edite antes de avancar.

### Sessao 2 — Plan

    "A spec esta aprovada. Gere o plan.md para specs/001-unlock-system/."

Agente gera plan.md com decisoes tecnicas.
Voce valida a abordagem, a ordem, os riscos.

### Sessao 3 — Tasks

    "Plan aprovado. Gere o tasks.md quebrando em tarefas atomicas."

Voce confere se as tarefas fazem sentido e se a ordem esta correta.

### Sessao 4 — Implementacao

    "Tasks aprovadas. Implemente seguindo specs/001-unlock-system/tasks.md.
    Siga a arquitetura hexagonal do CLAUDE.md."

Agente executa task por task, marcando [x] conforme conclui.

### Mudanca de requisito pos-implementacao

    1. Edite spec.md com a mudanca
    2. Peca ao agente para atualizar plan.md se necessario
    3. Peca ao agente para adicionar novas tasks em tasks.md
    4. Peca ao agente para atualizar codigo e testes

---

## Formato EARS para regras no spec.md

| Padrao           | Quando usar                   | Exemplo                                               |
|------------------|-------------------------------|-------------------------------------------------------|
| WHEN X -> Y      | Evento dispara comportamento  | WHEN jogador completa 10 partidas -> SHALL desbloquear "Veterano" |
| IF X -> Y        | Condicao de validacao         | IF nome estiver vazio -> retorna ErrInvalidName       |
| THE SYSTEM SHALL | Invariante sempre verdadeiro  | THE SYSTEM SHALL garantir unicidade de nome (ci)      |
| WHILE X, Y       | Comportamento continuo        | WHILE jogador na fila, SHALL notificar status a cada 30s |

---

## INSTRUCOES PARA O AGENTE DE IA

> Esta secao e direcionada ao Claude Code. Aplique em toda sessao neste repositorio.

### Antes de qualquer implementacao

1. Verifique se existe specs/NNN-<feature>/spec.md para o que esta sendo pedido
2. Se nao existir -> pergunte ao usuario antes de prosseguir. Nunca invente comportamento
3. Se existir spec mas nao houver plan -> oferea gerar plan.md antes de implementar
4. Se existir plan mas nao houver tasks -> oferea gerar tasks.md antes de implementar
5. Nunca pule fases sem confirmacao do usuario

### Ao gerar spec.md

- Use EARS para todas as regras de comportamento
- Seja especifico nos erros (nome do tipo, nao so "retorna erro")
- Liste todos os casos de teste obrigatorios — viram unit tests
- Preencha "Fora do escopo" para evitar scope creep
- NAO tome decisoes tecnicas no spec.md — isso vai no plan.md

### Ao gerar plan.md

- Liste exatamente quais arquivos serao criados ou modificados
- Se a decisao tecnica for arquiteturalmente significativa -> proponha ADR (docs/adr/ADR-GUIDE.md)
- Documente trade-offs considerados, nao so a decisao final

### Ao gerar tasks.md

- Cada task deve ser atomica: uma responsabilidade, um arquivo de destino
- Ordene por dependencia: domain antes de service, service antes de transport
- Inclua tasks de teste junto das tasks de implementacao (nao so no final)
- Use [ ] para tasks pendentes — marque [x] conforme implementar

### Durante a implementacao

- Execute task por task na ordem do tasks.md
- Marque [x] em cada task concluida antes de avancar para a proxima
- Se encontrar ambiguidade nao coberta pela spec -> pare e pergunte, nao chute
- Ao final: rode go test ./... -race e confirme que todos os casos da spec passam

### Nomenclatura de pastas

- Descubra o proximo numero inspecionando ls services/<nome>/specs/
- Formato: NNN-slug-kebab-case sem acentos
- Crie a pasta e os arquivos (spec, plan, tasks) na sequencia correta

---

## Prompts uteis por fase

Criar spec:
    "Me ajuda a escrever a spec de <feature> para o <servico>.
    Quero <descricao de intencao>. Use o formato do SDD-GUIDE.md."

Gerar plan a partir de spec aprovada:
    "A spec em specs/001-<feature>/spec.md esta aprovada.
    Gere o plan.md com as decisoes tecnicas de implementacao."

Gerar tasks a partir de plan aprovado:
    "Plan aprovado. Gere o tasks.md com tarefas atomicas ordenadas por dependencia."

Implementar:
    "Tasks aprovadas. Implemente seguindo specs/001-<feature>/tasks.md.
    Siga a arquitetura hexagonal descrita no CLAUDE.md."

Atualizar apos mudanca de requisito:
    "O requisito mudou: <descricao da mudanca>.
    Atualize specs/001-<feature>/spec.md e mostre o que muda no plan e nas tasks."

Verificar alinhamento spec vs codigo:
    "Revise o <servico> e verifique se o codigo implementa todos os casos
    da spec em specs/001-<feature>/spec.md. Liste divergencias encontradas."
