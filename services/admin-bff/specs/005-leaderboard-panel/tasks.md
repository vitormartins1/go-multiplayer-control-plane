# Tasks: 005-leaderboard-panel
**Plan:** specs/005-leaderboard-panel/plan.md
**Data:** 2026-06-30

## admin-bff
- [x] domain: `LeaderboardEntry` + Port `LeaderboardReader`; `ErrLeaderboardUnavailable`
- [x] adapter `internal/leaderboard` (+ httptest)
- [x] service: campo + `New` + `GameLeaderboard`
- [x] transport: rota `GET /api/games/{gameId}/leaderboard` + `writeUpstream` (502) + testes
- [x] main: `LEADERBOARD_URL` + client; compose

## admin-ui
- [x] `lib/api.ts`: `LeaderEntry` + `gameLeaderboard`
- [x] `pages/GameLeaderboards.tsx`; `GameDetail` usa na aba Leaderboards
- [x] `npm run build`

## verificação
- [x] `go test ./... -race` + `go build ./...`; rebuild Docker; abrir Games/(jogo)/Leaderboards
