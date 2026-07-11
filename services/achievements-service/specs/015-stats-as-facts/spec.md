# Spec 015 — Stats como facts nas event rules (etapa 2 do adendo ADR-0028)

**Status:** Done — implementado (`Facts.WithStats`/prefixo `stat.` em `internal/domain/event_rule.go`, aplicado em `ApplyMatchResult`/`ApplyPlacementResult`/`ApplyCollectiveResult`; host_world_dropin ficou de fora por escopo, conforme esta própria spec, e foi fechado depois pela spec `018-participation-stats`)
**Serviço:** achievements-service (domain + service); toque no admin-ui
(sugestões no canvas)
**Relaciona:** ADR-0028 adendo (roadmap de facts — esta etapa depende do V2a),
spec 014 (comparadores), spec 013 (fact `mode`, mesmo padrão de decorator)

## Contexto

Os envelopes de resultado já carregam `stats` por jogador (`kills`, `deaths`,
`survived_ms`…) — usadas hoje só pelo critério por métrica dos achievements.
Com os comparadores do V2a no motor, expor as stats como **facts numéricos**
destrava a classe de regra composta que era inexpressável: *flawless victory*
= `outcome = win` **e** `stat.deaths ≤ 0` → event type próprio → achievement.

## Comportamento

- **Prefixo `stat.`**: cada stat do jogador vira o fact `stat.<chave>` (valor
  numérico como string, ops do V2a comparam). O prefixo evita colisão com os
  facts estruturais (um jogo com stat chamada `outcome` não quebraria nada) e
  deixa a origem explícita no admin.
- **Decorator cross-kind** `Facts.WithStats(stats map[string]int64)` — mesmo
  padrão do `WithMode` (spec 013): as normalizadoras seguem só estruturais.
- **Onde entra** (por jogador):
  - `win_loss_draw` (queue_arena): stats do result de cada jogador;
  - `placement` (battle_royale): stats da entry **já com `survived_ms`
    injetado** (o mesmo mapa que alimenta o critério por métrica — regra
    `stat.survived_ms ≥ N` funciona);
  - `collective` (coop_mission): stats por jogador — **a avaliação de regras
    passa a ser por jogador** (hoje é uma vez por sessão, já que o outcome é
    coletivo; com stats individuais, cada jogador pode casar regras
    diferentes — ex.: só o MVP da missão ganha o event type).
  - `participation_event` (host_world_dropin): fora — o envelope não carrega
    stats (limitação de contrato registrada no adendo; entra na fatia
    cross-repo se houver demanda).
- Stat ausente no envelope: fact não existe → regra com `stat.X` não casa
  (semântica padrão do motor, nada especial).
- **Admin-ui (canvas)**: o Autocomplete de chave sugere `stat.<s>` para cada
  stat do `scoringStats` dos modos do arquétipo selecionado (mesmo mecanismo
  da sugestão de `mode`, spec 013). Nada mais muda — condições/nós já são
  genéricos desde o V2a.

## Fora de escopo

- Stats no envelope de participação (contrato cross-repo).
- Agregações (soma do time, média) — os facts são as stats brutas por jogador.
- V2b (supressão de global) e V2c (prioridade).

## Critérios de aceite

1. **Flawless victory de ponta a ponta** (e2e via SQS, método das specs
   013/014): regra `{outcome: win, stat.deaths: {op: lte, value: 0}}` →
   envelope win com `deaths: 0` casa; win com `deaths: 2` não; loss com
   `deaths: 0` não.
2. No coop, regra com `stat.<x>` casa só pros jogadores cujo stat satisfaz —
   os demais seguem recebendo apenas os event types coletivos.
3. Zero regressão: suíte atual passa sem mudança de expectativa; regras sem
   `stat.*` avaliam idêntico (inclusive a otimização de avaliar uma vez no
   coop só é trocada quando há stats envolvidas — decisão no plan).
4. `go test ./... -race` verde; build do admin-ui verde.
