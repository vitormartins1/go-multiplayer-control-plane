# Tasks: 005-team-composition

- [x] **T1** — `internal/domain/matchmaking.go`: `MatchFormat{ Size, Structure, TeamCount, TeamSize }`; `TeamAssignment{ TeamID, Players }`; `ProfileProvider.Format(ctx, gameID) (MatchFormat, error)` (remove `MatchSize`); `SessionProvisioner.Provision(ctx, matchID string, teams []TeamAssignment, gameID string)`.
- [x] **T2** — `internal/registryclient/client.go`: implementa `Format` — deriva `Size` (ffa/teams) e devolve `Structure/TeamCount/TeamSize`; `ErrProfileUnavailable` inalterado.
- [x] **T3** — `internal/profilecache/cache.go`: cacheia `MatchFormat` (entry com format), método `Format`.
- [x] **T4** — `internal/service/matchmaker.go`: `ticket.format`; `Join` chama `Format`; `tryForm` agrupa por `format.Size` e **particiona** (blocos contíguos, `team-<n>`); `provision` monta `[]TeamAssignment` (ffa = 1 assignment sem teamID) e chama `Provision`.
- [x] **T5** — `internal/sessionclient/grpc_client.go`: `Provision` monta `CreateSessionRequest{ players: união dos teams, teams: assignments (omitir em ffa) }`.
- [x] **T6** — Testes service: particionamento 2×1 / 2×2 / 3×3 (times corretos, rótulos estáveis) e ffa (sem times); `Join` com `Format`; mock do provisioner recebe os `teams` esperados.
- [x] **T7** — Testes registryclient/profilecache: `Format` deriva certo; cache devolve format.
- [x] **T8** — `go build ./... && go test ./services/matchmaking-service/... -race -cover`; `golangci-lint run`.
- [x] **T9** — Aprendizado (CLAUDE.md): GO-LEARNINGS se houver (ex.: particionamento de slice / evolução de Port sem quebrar o cache decorator); senão dizer que não houve.
