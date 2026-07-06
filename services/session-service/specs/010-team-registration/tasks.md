# Tasks: 010-team-registration

- [x] **T1** — `proto/session/v1/session.proto`: `message Team{ string team_id=1; repeated string players=2 }`; `repeated Team teams = 4` em `CreateSessionRequest`. `buf generate` (de `services/session-service/`); conferir `gen/...`.
- [x] **T2** — `internal/domain/session.go`: `TeamAssignment{ TeamID; Players }`; `Session.TeamByPlayer map[string]string`; `SessionService.Create(ctx, matchID, gameID string, teams []TeamAssignment)`; `ConnectTokenIssuer.Issue(playerID, matchID, serverAddr, teamID string)`.
- [x] **T3** — `internal/domain/errors.go`: `ErrInvalidTeams`.
- [x] **T4** — `internal/token/connect_issuer.go`: `Issue` ganha `teamID`; adiciona claim `team_id` só se não vazio. Ajustar `connect_issuer_test.go` (assinatura + caso com/sem team).
- [x] **T5** — `internal/service/session_service.go`: `Create` recebe `teams`, valida composição (→ `ErrInvalidTeams`), deriva `players`+`teamByPlayer`, emite tokens com o teamID de cada jogador, grava `TeamByPlayer` na sessão.
- [x] **T6** — `internal/service/session_service.go`: `ReportResult` carimba `p.TeamID` a partir de `sess.TeamByPlayer` antes do `Publish`.
- [x] **T7** — `internal/transport/grpc_server.go`: `CreateSession` monta `[]TeamAssignment` (usa `teams` se presente; senão um assignment FFA com `players`); mapeia `ErrInvalidTeams` → `InvalidArgument`.
- [x] **T8** — Testes service: `Create` com times registra `TeamByPlayer` e emite token por time; `ReportResult` carimba (sobrescreve o que o server mandou); FFA sem regressão; composição inconsistente → `ErrInvalidTeams`.
- [x] **T9** — Testes transport (bufconn): CreateSession com `teams`; ReportResult inalterado.
- [x] **T10** — `go build ./... && go test ./services/session-service/... -race -cover`; `golangci-lint run`.
- [x] **T11** — Aprendizado (CLAUDE.md): registrar em GO-LEARNINGS se houver algo novo (ex.: claim opcional no JWT / mutação in-place via ponteiro do oneof); senão dizer que não houve.
