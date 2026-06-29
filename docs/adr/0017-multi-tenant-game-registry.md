# ADR-0017: Multi-tenant por jogo — game-registry + dimensão `game_id`

**Status:** Accepted
**Data:** 2026-06-29
**Serviço(s) afetado(s):** game-registry (novo), achievements, admin-bff, admin-ui, infra
**Relaciona / operacionaliza:** ADR-0015 (#4 multi-tenant por `game_id`); ADR-0009 (identity global); ADR-0008/0005 (tese de custo)

---

## Contexto

O ADR-0015 (#4) decidiu o **princípio**: "vários jogos" é uma **dimensão (`game_id`) dentro do serviço, não um banco por jogo". Mas não operacionalizou: falta (a) onde mora a fonte canônica de jogos, (b) como o jogador se relaciona com jogos, (c) como os domínios recebem o `game_id`. O objetivo prático: **rodar vários jogos no mesmo backend** e ter no painel admin um jeito **concreto de cadastrar um jogo de teste** e seus achievements.

## Alternativas Consideradas

### Opção A: game-registry + `game_id` como dimensão, identidade global (escolhida)
- **Prós:** um control plane serve N jogos; barato (alinha ADR-0008/0005); setup de jogo de teste trivial; respeita database-per-service.
- **Contras:** introduzir `game_id` nos domínios é migração incremental cross-service; toda query por-jogo precisa filtrar por `game_id` (disciplina).

### Opção B: infra-por-jogo (stack dedicada por jogo)
- **Contras:** multiplica deploy/banco/observabilidade/custo; contradiz a tese (ADR-0008) e o ADR-0015. **Rejeitada.**

### Opção C: registro de jogos dentro de um serviço existente (identity/admin-bff)
- **Contras:** identity é o **jogador** (ADR-0009), admin-bff é **stateless** (ADR-0016). Misturar responsabilidades. **Rejeitada.**

### Opção D: conta por jogo (`game_id` em `accounts`)
- **Contras:** quebra a identidade global do ADR-0009; duplica o jogador por jogo. **Rejeitada.**

## Decisão

1. **Novo serviço `game-registry`** (Go, hexagonal, Postgres próprio) — dono da tabela `games` (`id` uuid, `slug` único, `name`, `created_at`). Expõe registrar/listar jogos (admin, interno, sem auth no MVP — como os demais `/admin`).
2. **Identidade global** (ADR-0009 inalterado): uma conta serve vários jogos. "Quais jogos um user joga" **emerge dos dados por-jogo** dele (tem achievements no jogo A, ranking no B…), não de um vínculo no `accounts`.
3. **`game_id` é uma dimensão** nos domínios por-jogo, adotada **incrementalmente, um por feature**. **Achievements primeiro**: o catálogo e o progresso passam a ter `game_id`. Leaderboard/player-data/cloud-save adotam depois, cada um em sua feature.
4. **O `game_id` é explícito do chamador** (o cliente/Unity sabe qual jogo é) — passado por parâmetro/rota/header nas chamadas por-jogo; **não** é derivado do token global do jogador. O mecanismo exato (header vs path vs claim) fica no `plan.md` de cada serviço.
5. **Integridade entre serviços por `id` + API** (sem FK cross-service, ADR-0015): achievements guarda `game_id`, não faz JOIN em `games`; valida o jogo via API do registry quando necessário.
6. **Painel admin ganha a seção "Games"**: registrar/listar jogos e, por jogo, cadastrar o catálogo de achievements — a ferramenta concreta de setup de jogo de teste.

## Consequências

### Positivas
- Um backend para N jogos (barato, alinhado à tese); setup de jogo de teste fácil pelo painel; database-per-service preservado.

### Negativas / Trade-offs
- `game_id` entra por migração incremental em vários serviços ao longo do tempo; toda leitura/escrita por-jogo precisa filtrar por `game_id` — um `game_id` ausente é um footgun (validar sempre).

### Neutras / Implicações
- **Migração do achievements:** o `game_id` entra numa tabela que já tem dados (de teste). A migração cria um **jogo "default"** e atribui os registros existentes a ele (preserva o que há, sem reset).
- Achievements é a primeira ponta; os demais domínios entram quando o painel/uso pedir.
- O `game-registry` tem seu próprio Postgres (container) e entra na observabilidade como qualquer serviço novo (RED, scrape, exporter — ver docs/OBSERVABILITY.md).
- Decorre uma cadeia de features SDD: `game-registry/001`, `achievements/004-game-scope`, `admin-bff/004-games`, `admin-ui/003-games`.

## Referências
- ADR-0015 (#4 multi-tenant por `game_id`), ADR-0009 (identity global), ADR-0008/0005 (custo), ADR-0016 (painel admin)
- docs/OBSERVABILITY.md (serviço novo entra na observabilidade)
