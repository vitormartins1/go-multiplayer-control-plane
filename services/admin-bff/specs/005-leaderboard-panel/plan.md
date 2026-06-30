# Plan: 005-leaderboard-panel
**Spec:** specs/005-leaderboard-panel/spec.md
**Data:** 2026-06-30

## admin-bff
- `internal/domain/admin.go` — `LeaderboardEntry{Rank,PlayerID,Score}`; Port `LeaderboardReader.TopForGame(ctx, gameID, limit)`
- `internal/domain/errors.go` — `ErrLeaderboardUnavailable`
- `internal/leaderboard/client.go` (+ test httptest) — adapter espelhando o de achievements (do() classifica; unavailable -> ErrLeaderboardUnavailable)
- `internal/service/admin_service.go` — campo `leaderboard`; `New(...)` ganha o reader; `GameLeaderboard(ctx, gameID, limit)`
- `internal/transport/http_handler.go` — interface `AdminService` + rota `GET /api/games/{gameId}/leaderboard`; handler lê `limit`, `writeUpstream` trata `ErrLeaderboardUnavailable` (502)
- `internal/transport/http_handler_test.go` + `internal/service/admin_service_test.go` — fakes/New atualizados; 200 + 502
- `cmd/server/main.go` — `LEADERBOARD_URL` + `leaderboardclient.New`; compose `LEADERBOARD_URL`

## admin-ui
- `src/lib/api.ts` — tipo `LeaderEntry`; `api.gameLeaderboard(gameId, limit?, signal)`
- `src/pages/GameLeaderboards.tsx` — `useAsync` + tabela (rank/jogador/score) + Atualizar + estados vazio/erro
- `src/pages/GameDetail.tsx` — aba Leaderboards usa `<GameLeaderboards gameId>` (era ComingSoon)

## Decisões
- **Read-only** no painel (submit é via fan-out server-side).
- Reusa o padrão de adapter/UpstreamError; mesma classificação 4xx/5xx.
- `limit` default no service do leaderboard (não precisa no BFF).

## Ordem
1. bff: domain -> adapter+test -> service -> transport -> main/compose -> go test+build
2. ui: api -> GameLeaderboards -> GameDetail -> npm build
3. Docker: gerar scores (fan-out) -> abrir Games/(jogo)/Leaderboards no painel

## Riscos
- **Churn de testes** no service/transport do bff (New + fakeService ganham método).
- Garantir `writeUpstream` cobrir o novo erro (502).
