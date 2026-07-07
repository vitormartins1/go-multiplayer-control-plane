# Plan: 017-mode-snapshot-manifest

**Spec:** `spec.md` (aprovada 2026-07-07) · **ADR:** 0024 · **Fatia:** M1 (núcleo)

## Decisões técnicas

1. **`ProfileSnapshot` substitui `WorldProfile` como o tipo do perfil no domain.** Um único tipo serve os 4 arquétipos: `{ModeID, Archetype, Topology, ParamsJSON []byte, Hash string}` + os derivados do host_world (`EnabledRoles/CoopSlots/InvaderSlots`, zero nos demais). A Port vira `ProfileProvider.ModeProfile(ctx, gameID, modeID)`. O `registryclient` busca `GET /games/{id}/multiplayer-profiles/{modeId}`, guarda o corpo cru dos `params` (manifest) e calcula o hash (SHA-256 truncado a 16 hex do corpo do modo como recebido) — crypto no adapter, domain puro.

2. **Snapshot obrigatório na criação.** `Create`/`OpenWorld` ganham `modeID` na assinatura e falham com `ErrProfileUnavailable` se o perfil do modo não puder ser lido (`profiles == nil` = erro de wiring, mesmo tratamento). A sessão guarda `ModeID` + `Profile *ProfileSnapshot`. Normalização `"" → "default"` só no transport.

3. **Cache TTL decorator** (`internal/profilecache`, espelho do matchmaking, GO-LEARNINGS #21), chaveado por `(gameID, modeID)`, montado no main entre o registryclient e o service.

4. **Validação kind×arquétipo** no `ReportResult`, antes do publish: mapa `archetype → kind esperado` (`queue_arena→win_loss_draw`, `battle_royale→placement`, `coop_mission→collective`; `host_world_dropin` → sempre mismatch). Constantes de arquétipo entram no domain. Erro `ErrOutcomeMismatch` → `FailedPrecondition`.

5. **`GetSessionConfig(ref_id)`**: resolve por `matchIndex` e, se ausente, por `sessionID` (mesma rechaveação do restante); sessão fechada/inexistente → `ErrSessionNotFound`. Devolve `domain.SessionManifest` montado do snapshot.

6. **Publishers ganham `modeID` na assinatura** (`Publish(ctx, gameID, matchID, modeID, outcome)` e `PublishParticipation(..., modeID, evt)`); envelopes ganham `"modeId"`, `schemaVersion` **permanece 2** (aditivo).

7. **Token:** `ConnectClaims.ModeID`, claim `mode_id` **sempre** emitido (Create/OpenWorld/JoinWorld — o JoinWorld lê `sess.ModeID`).

8. **Churn de testes é aceito e mecânico:** assinaturas de `Create`/`OpenWorld`/`Publish` mudam; os mocks/arranges são atualizados com um stub de perfil default (`queue_arena`) e um `mock.Anything` extra no Publish.

## Arquivos

| Arquivo | Mudança |
|---|---|
| `proto/session/v1/session.proto` + `gen/` | `mode_id` em Create/OpenWorld; RPC `GetSessionConfig`; `buf generate` |
| `internal/domain/session.go` | `ProfileSnapshot`, `SessionManifest`, `Session.ModeID/Profile`, arquétipos, Ports (`ModeProfile`, `Create/OpenWorld` c/ modeID, `GetSessionConfig`, publishers c/ modeID, `ConnectClaims.ModeID`) |
| `internal/domain/errors.go` | `ErrOutcomeMismatch` |
| `internal/registryclient/client.go` | `ModeProfile` por URL da coleção + params crus + hash |
| `internal/profilecache/cache.go` | novo decorator TTL por (game, mode) |
| `internal/service/session_service.go` | snapshot no Create, validação kind×arquétipo, `GetSessionConfig` |
| `internal/service/host_world.go` (ou onde vive OpenWorld/JoinWorld) | modeID/snapshot/claims |
| `internal/token/connect_issuer.go` | claim `mode_id` |
| `internal/publisher/{sns,log}_publisher.go` | `modeId` nos 4 envelopes |
| `internal/transport/grpc_server.go` | normalização default, campos novos, `GetSessionConfig`, mismatch→FailedPrecondition |
| `internal/transport/http_handler.go` | `modeId` no board admin |
| `cmd/server/main.go` | wiring do cache decorator |
| `docs/game-models/*-dataplane-contract.md` (4) | claim `mode_id` + seção do manifest |

## Riscos / atenção

- Regenerar proto **sempre via `buf`** (rawDesc).
- `OpenWorld` deixa de validar arquétipo no adapter (registryclient genérico) — a checagem `archetype == host_world_dropin` sobe para o service (mesmo erro externo `ErrUnsupportedArchetype`).
- admin-bff/admin-ui: o campo `modeId` no board é aditivo — BFF repassa struct tipado (`domain.GameSession`)? Conferir: se o BFF tipa a sessão, adicionar o campo lá também (aditivo, sem UI nova).
