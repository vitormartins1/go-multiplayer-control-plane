# Spec: 009-polymorphic-result

**Serviço:** session-service (+ contrato do fan-out com os consumidores)
**Status:** Done (2026-07-06)
**Data:** 2026-07-06
**ADR:** 0023 (perfil de multiplayer) — **fatia 3** (resultado polimórfico), relaciona ADR-0020 (fan-out)

## Contexto

Terceira fatia do ADR-0023, e o **gargalo central** que as 5 fichas de `docs/game-models/` apontaram: o resultado da partida é hoje `PlayerResult{ PlayerID; Score int64; Won bool }` (domínio, gRPC `ReportResult`, envelope de fan-out — ADR-0020). Esse formato só expressa "venceu/não venceu por jogador" — não cobre **empate**, **stats ricas** (kills/assists/…), **times**, nem os outros modelos de outcome (colocação, coletivo, por papel).

Esta fatia generaliza o resultado no **session-service** (produtor): domínio, contrato gRPC `ReportResult` e **envelope do fan-out**. Implementa o tipo de outcome **`win_loss_draw`** (o que o arquétipo `queue_arena` precisa: WIN/LOSS/**DRAW** + stats + times), deixando o modelo **extensível** para `placement` (battle_royale), `collective` (coop_mission) e `role_event` (host_world_dropin) — não implementados aqui.

O fan-out permanece **retrocompatível** (ADR-0020): o envelope mantém `won`/`score`, então os consumidores atuais (achievements, leaderboard, analytics) **seguem funcionando** sem mudança; eles passam a ler os campos ricos em specs próprias (fatias seguintes).

## Comportamento

### Reportar o resultado (outcome polimórfico)

- WHEN o game server chama `ReportResult{matchId, outcome}` com um outcome **`win_loss_draw`** válido → THE SYSTEM SHALL publicar o fan-out do resultado, liberar o game server e encerrar a sessão (`CloseResult`) — **fluxo inalterado**, só o formato do resultado muda.
- IF não existir sessão para o `matchId` → `NotFound` (**ErrSessionNotFound**) — inalterado.
- IF o `outcome.kind` não for suportado (no MVP, só `win_loss_draw`) → `InvalidArgument` (**ErrUnsupportedOutcome**).
- IF o outcome for estruturalmente inválido (sem jogadores; `outcome` de um jogador fora de `{win,loss,draw}`) → `InvalidArgument` (**ErrInvalidOutcome**).
- THE SYSTEM SHALL **não** validar o outcome contra o perfil do jogo (o game server é autoritativo sobre o resultado). Ex.: um `draw` é aceito mesmo que o perfil tenha `allowDraw=false` — a coerência é responsabilidade do data plane.

### Forma do outcome `win_loss_draw`

Por jogador: `outcome` (`win`|`loss`|`draw`), `score`, `teamId` (vazio em FFA) e `stats` (métricas declaradas, ex.: `{"kills":7,"assists":3}`). O **outcome por time** (quando há `teamId`) é o outcome compartilhado pelos jogadores do time — derivável, não duplicado.

### Fan-out (envelope enriquecido e retrocompatível)

- WHEN publica o resultado → THE SYSTEM SHALL emitir o envelope com os campos **novos** por jogador (`outcome`, `teamId`, `stats`) **e manter** `won`/`score` para os consumidores atuais. `won` deriva do outcome (`won = (outcome == "win")`).
- THE SYSTEM SHALL incrementar `schemaVersion` para `2` (sinaliza os campos novos; consumidores ignoram o que não usam — ADR-0020).
- A idempotência por `matchId` e a durabilidade (SNS→SQS) permanecem inalteradas.

## Contratos de Interface

### gRPC (`session.v1`)

`ReportResultRequest` passa a carregar o outcome polimórfico (em vez de `repeated PlayerResult`). O desenho exato do `.proto` (oneof por kind vs. envelope + campos) é decisão do **plan**. O contrato **muda** — o data plane (repo Unity) adota o novo formato quando implementar o gameplay (spec dataplane `006-match-result` será atualizada; o projeto Unity ainda não existe, então não há quebra em produção).

### Ports (domínio)

- `SessionService.ReportResult(ctx, matchID string, outcome MatchOutcome) error` — substitui `results []PlayerResult`.
- `ResultPublisher.Publish(ctx, gameID, matchID string, outcome MatchOutcome) error` — passa a receber o outcome rico.

### Tipos (domínio)

- `MatchOutcome{ Kind string; WinLossDraw *WinLossDrawResult }` — **tagged union por ponteiro** (mesmo padrão do game-registry, GO-LEARNINGS #20). `Kind` = `"win_loss_draw"` no MVP; novos kinds adicionam um campo-ponteiro.
- `WinLossDrawResult{ Players []PlayerOutcome }`.
- `PlayerOutcome{ PlayerID string; TeamID string; Outcome string; Score int64; Stats map[string]int64 }`.
- `PlayerResult{Score,Won}` é **removido** do domínio (substituído por `PlayerOutcome`).

## Casos de Teste Obrigatórios

- [ ] `ReportResult` com `win_loss_draw` FFA (win/loss) → publica; libera server; encerra `CloseResult`
- [ ] `ReportResult` com **empate** (`draw`) → publica outcome=`draw`; no envelope `won=false` (compat)
- [ ] `ReportResult` com **times** (`teamId` preenchido) → envelope carrega `teamId` por jogador
- [ ] `ReportResult` com **stats** → envelope carrega `stats`; stats vazio também é válido
- [ ] envelope: `schemaVersion=2`, campos novos presentes **e** `won`/`score` mantidos (retrocompat)
- [ ] `outcome.kind` não suportado → `ErrUnsupportedOutcome`; outcome inválido (sem players / valor fora de win/loss/draw) → `ErrInvalidOutcome`
- [ ] `matchId` inexistente → `ErrSessionNotFound` (inalterado)
- [ ] falha de publicação propaga (sessão não encerra) — comportamento do fan-out inalterado (ADR-0020)

## Fora do escopo

- **Consumidores** (achievements/leaderboard/analytics) lendo os campos novos (`outcome`/`stats`/`team`) — specs próprias; aqui eles seguem via `won`/`score`.
- **Outros kinds** de outcome (`placement`, `collective`, `role_event`) — só o slot no modelo; implementação quando os arquétipos existirem.
- **Composição de times no matchmaking** (quem joga em qual lado) — fatia irmã.
- **Validação do outcome contra o perfil** (allowDraw, scoringStats declaradas) — o game server é autoritativo; validação cruzada é evolução.
- **Atualização do projeto Unity** (data plane) — outro repo; adota o novo `.proto` quando implementar o gameplay.
