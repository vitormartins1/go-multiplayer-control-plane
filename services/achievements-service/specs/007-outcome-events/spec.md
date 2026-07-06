# Spec: 007-outcome-events

**Serviço:** achievements-service (consumidor do fan-out)
**Status:** Implementada (2026-07-06)
**Data:** 2026-07-06
**ADR:** 0023 (resultado polimórfico) — **fatia 3, consumidor**; relaciona ADR-0020 (fan-out)

## Contexto

A fatia 3 (session `009-polymorphic-result`) enriqueceu o envelope do fan-out com o **outcome** por jogador (`win`/`loss`/`draw`), stats e times — mantendo `won`/`score` para retrocompat. Hoje o achievements lê só `won` e gera `match_played` (todos) + `match_won` (quem venceu) — então **empate e derrota são invisíveis** (um empate chega como `won=false`, indistinto de derrota). Esta fatia faz o achievements **ler o `outcome`** e gerar também **`match_lost`** e **`match_drawn`**, destravando conquistas ligadas a derrota/empate.

É a primeira colheita do resultado rico: valida ponta a ponta a retrocompatibilidade (o consumidor migra de `won` para `outcome`, com fallback). Fica **fora** o uso de `stats`/`teamId` (conquistas por métrica/time — evoluções) e os outros consumidores (leaderboard, analytics).

## Comportamento

### Traduzir o outcome em eventos

- WHEN o fan-out entrega um resultado com `outcome` presente por jogador → THE SYSTEM SHALL gerar, por jogador: **`match_played`** (sempre) e o evento do outcome — `win` → **`match_won`**, `loss` → **`match_lost`**, `draw` → **`match_drawn`**.
- IF o `outcome` estiver **ausente** (envelope antigo/schemaVersion 1, ou campo vazio) → THE SYSTEM SHALL cair no comportamento atual via `won`: `won=true` → `match_played` + `match_won`; `won=false` → só `match_played`. (Retrocompat — nenhum evento novo sem `outcome`.)
- IF o `outcome` tiver um valor desconhecido (nem win/loss/draw) → THE SYSTEM SHALL gerar só `match_played` (ignora o valor inválido, não falha o processamento).
- THE SYSTEM SHALL manter a **idempotência por matchID** inalterada (reprocessar a mesma partida é no-op — ADR-0020).

## Contratos de Interface

- **Consumer** (`cmd/consumer/main.go`): o struct do envelope ganha `Outcome string` por resultado (além de `won`, mantido para o fallback).
- **Domain** (`internal/domain/achievement.go`): `MatchResult` ganha `Outcome string` (mantém `Won`).
- **Service** (`ApplyMatchResult`): mapeia `Outcome`→evento com fallback para `Won`; novos eventos `match_lost`, `match_drawn` (constantes).
- **Catálogo:** sem mudança de schema — os event types são strings; conquistas podem ser configuradas para `match_lost`/`match_drawn` (que agora passam a ser emitidos). Nenhuma migração.

## Casos de Teste Obrigatórios

- [ ] `outcome=win` → `match_played` + `match_won`
- [ ] `outcome=loss` → `match_played` + `match_lost`
- [ ] `outcome=draw` → `match_played` + `match_drawn`
- [ ] `outcome` ausente + `won=true` → `match_played` + `match_won` (retrocompat)
- [ ] `outcome` ausente + `won=false` → só `match_played` (retrocompat, como hoje)
- [ ] `outcome` inválido (ex.: "victory") → só `match_played` (não falha)
- [ ] idempotência: reprocessar o mesmo matchID → `dup=true`, no-op (inalterado)
- [ ] (consumer) desserializa o envelope v2 com `outcome` e repassa ao service

## Fora do escopo

- **Conquistas por `stats`** (ex.: "10 kills numa partida") — exige eventos com valor; evolução (relaciona spec 002-partial-progress).
- **`teamId`** — conquistas por time; evolução.
- **leaderboard / analytics** lendo os campos ricos — specs próprias.
- Popular o catálogo com conquistas de derrota/empate — isso é dado (admin), não código.
