# Spec: 003-game-scope

**Serviço:** matchmaking-service (+ companion no session-service)
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Faz o **`game_id` fluir pelo match** (ADR-0017), pré-requisito do fan-out do resultado. O cliente informa o jogo no `FindMatch` (mesmo padrão "contexto do cliente" do `X-Game-Id` REST); o matchmaking passa a **formar partidas por jogo** (nunca mistura jogos) e propaga o `game_id` ao `CreateSession`; o session **guarda** o `game_id` na sessão (o `ReportResult`/fan-out usará — peça 4).

## Comportamento

### Matchmaking
- WHEN `FindMatch{game_id}` (autenticado) -> enfileira o ticket do jogador **no jogo `game_id`**.
- IF `game_id` vazio -> `InvalidArgument`.
- THE SYSTEM SHALL formar matches **apenas entre jogadores do mesmo `game_id`** (FIFO por jogo); nunca agrupa jogadores de jogos diferentes.
- WHEN um match forma -> chama `CreateSession{match_id, players, game_id}` (provisiona com o jogo).

### Session (companion)
- WHEN `CreateSession{match_id, players, game_id}` -> registra a sessão **com o `game_id`** (além de server + tokens). O `game_id` fica disponível para o `ReportResult` (peça 4).

## Contratos de Interface

### Protobuf
```
// matchmaking
message FindMatchRequest { string game_id = 1; }
// session
message CreateSessionRequest { string match_id = 1; repeated string players = 2; string game_id = 3; }
```

### Ports (domínio)
- matchmaking `Matchmaker.Join(ctx, gameID, playerID)`; `SessionProvisioner.Provision(ctx, matchID, players, gameID)`
- session `SessionService.Create(ctx, matchID, players, gameID)`; `Session` ganha `GameID`

## Casos de Teste Obrigatórios

- [ ] Dois jogadores no **mesmo** `game_id` -> formam match; o `game_id` chega ao `Provision`/`CreateSession`
- [ ] Jogadores em **jogos diferentes** não formam match entre si
- [ ] `FindMatch` sem `game_id` -> `InvalidArgument`
- [ ] (session) `CreateSession` guarda o `game_id` na sessão

## Fora do escopo

- **Fan-out do `ReportResult`** (usa o `game_id` guardado) — peça 4 (session)
- Skill/região por jogo — backlog
