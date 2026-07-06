# Plan: 010-team-registration

**Spec:** `spec.md` (Aprovada) · **ADR:** 0023

## Decisões técnicas

### D1 — Proto: `teams` aditivo no `CreateSessionRequest`
Adiciona `repeated Team teams = 4;` com `Team{ string team_id = 1; repeated string players = 2; }`. Mantém `repeated string players = 2` como a **lista completa** de jogadores (fonte para alocação/tokens e retrocompat). FFA: chamador manda só `players`, omite `teams`. `buf generate` (de `services/session-service/`) regenera `gen/...`.

### D2 — Domínio: composição por `TeamAssignment` + `TeamByPlayer` na sessão
- Novo `TeamAssignment{ TeamID string; Players []string }`.
- `SessionService.Create` muda de `(…, players []string)` para `(…, teams []TeamAssignment)`. FFA = um assignment com `TeamID:""` e todos os players. O service **deriva** `players` (união, ordem preservada) e `teamByPlayer map[string]string`.
- `Session` ganha `TeamByPlayer map[string]string` (nil/vazio em FFA). `Players []string` mantido.
- Novo erro `ErrInvalidTeams` (composição inconsistente).

### D3 — Token carrega `team_id`
`ConnectTokenIssuer.Issue(playerID, matchID, serverAddr, teamID string)`. O claim `team_id` é **omitido quando vazio** (FFA) para não poluir o token. Teste do issuer cobre presença (teams) e ausência (ffa).

### D4 — Carimbo no `ReportResult`
No service, após validar o outcome e localizar a sessão, **sobrescrever** `p.TeamID = sess.TeamByPlayer[p.PlayerID]` para cada `PlayerOutcome` **antes** de `Publish`. `WinLossDraw` é ponteiro → mutação in-place do slice. Jogador desconhecido da sessão: mantém o comportamento atual (não está no mapa → TeamID vazio; a validação de outcome da spec 009 não muda).

### D5 — Validação da composição (transport → service)
O transport monta `[]TeamAssignment` do request (se `teams` vazio → um assignment FFA com `players`). O **service** valida (defesa em profundidade): todo player de `teams` está em `players` e vice-versa; nenhum player em dois times. Inconsistência → `ErrInvalidTeams` → transport mapeia para `InvalidArgument`.

## Arquivos

| Arquivo | Mudança |
|---|---|
| `proto/session/v1/session.proto` | `Team` message; `repeated Team teams = 4` em `CreateSessionRequest`. `buf generate` |
| `internal/domain/session.go` | `TeamAssignment`; `Session.TeamByPlayer`; `Create` assinatura; `ConnectTokenIssuer.Issue` assinatura |
| `internal/domain/errors.go` | `ErrInvalidTeams` |
| `internal/token/connect_issuer.go` | `Issue` emite claim `team_id` (omitido se vazio) |
| `internal/service/session_service.go` | `Create` recebe teams, deriva players/teamByPlayer, valida, emite token c/ teamID; `ReportResult` carimba |
| `internal/transport/grpc_server.go` | `CreateSession` monta `[]TeamAssignment` (teams ou FFA); mapeia `ErrInvalidTeams` |
| `cmd/server/main.go` | ajustar chamada do `Issue` se houver call direto (há só via service) |
| testes | issuer (claim), service (create c/ times, carimbo, ffa, inválido), transport (bufconn) |

## Ordem
proto → gen → domain → errors → issuer → service → transport → testes → build/`-race`/lint.
