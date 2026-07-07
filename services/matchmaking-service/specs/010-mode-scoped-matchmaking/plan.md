# Plan: 010-mode-scoped-matchmaking

**Spec:** `spec.md` (aprovada 2026-07-07) · **ADR:** 0024 · **Fatia:** M1 (lado matchmaking)

## Decisões técnicas

1. **Normalização na borda:** `"" → "default"` uma vez no transport (`grpc_server.go`), em todas as entradas (`FindMatch`, `Invade`, `PostSummonSign`, `ListSigns`). Domain/service sempre recebem modeID não-vazio.
2. **Ports ganham `modeID`:** `ProfileProvider.Format(ctx, gameID, modeID)`, `InvasionPolicyProvider.InvasionPolicy(...)`, `CoopPolicyProvider.CoopPolicy(...)`, `SessionProvisioner.Provision(ctx, matchID, teams, gameID, modeID)`.
3. **registryclient:** um único `getProfile(ctx, gameID, modeID)` na URL da coleção (`/multiplayer-profiles/{modeId}`); abandona o alias legado.
4. **profilecache:** chave composta `key{gameID, modeID}` (struct como chave de map — comparável), semântica/TTL inalterados.
5. **Fila:** `ticket` ganha `modeID`; `tryForm` agrupa por `key{gameID, modeID}`; `provision` repassa `group[0].modeID`; `sessionclient` envia `mode_id` no `CreateSessionRequest` (proto do session regenerado na spec `session/017`).
6. **Sinais:** o sinal guarda `modeID` (do aliado); `ListSigns(gameID, modeID, faixa)` filtra por jogo+modo. `Invade` lê a política do modo e injeta normalmente (o diretório de invadíveis não filtra por modo no M1 — mundos são por modo via session, e a guarda dura é follow-up declarado).

## Arquivos

| Arquivo | Mudança |
|---|---|
| `proto/matchmaking/v1/matchmaking.proto` + `gen/` | `mode_id` nos 4 requests; `buf generate` |
| `internal/domain/matchmaking.go` | assinaturas das Ports com `modeID` |
| `internal/registryclient/client.go` | URL da coleção; `Format/InvasionPolicy/CoopPolicy` c/ modeID |
| `internal/profilecache/cache.go` | chave (game, mode) |
| `internal/service/matchmaker.go` | ticket/agrupamento/provision c/ modeID |
| `internal/service/invader.go` / `summoner.go` | política por modo; sinal c/ modeID; filtro no ListSigns |
| `internal/sessionclient/grpc_client.go` | `mode_id` no CreateSessionRequest |
| `internal/transport/grpc_server.go` | normalização + repasse |

## Riscos / atenção

- Depende do stub regenerado do session (fazer a T1 do session/017 antes).
- Invade sem filtro de modo no diretório: comportamento documentado (mundo aberto em modo X pode receber invasor que pediu modo Y **se** a política do modo Y permitir — mitigado porque a política vem do modo e a guarda dura é follow-up). Manter nota no código.
