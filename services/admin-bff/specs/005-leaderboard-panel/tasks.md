# Tasks: 005-leaderboard-panel
**Plan:** specs/005-leaderboard-panel/plan.md
**Data:** 2026-06-30

## admin-bff
- [ ] domain: `LeaderboardEntry` + Port `LeaderboardReader`; `ErrLeaderboardUnavailable`
- [ ] adapter `internal/leaderboard` (+ httptest)
- [ ] service: campo + `New` + `GameLeaderboard`
- [ ] transport: rota `GET /api/games/{gameId}/leaderboard` + `writeUpstream` (502) + testes
- [ ] main: `LEADERBOARD_URL` + client; compose

## admin-ui
- [ ] `lib/api.ts`: `LeaderEntry` + `gameLeaderboard`
- [ ] `pages/GameLeaderboards.tsx`; `GameDetail` usa na aba Leaderboards
- [ ] `npm run build`

## verificação
- [ ] `go test ./... -race` + `go build ./...`; rebuild Docker; abrir Games/(jogo)/Leaderboards
