# Tasks: 006-player-tabs
**Plan:** specs/006-player-tabs/plan.md
**Data:** 2026-06-30
**Pré-requisito:** player-data/002-game-scope + cloud-save/002-game-scope

## admin-bff
- [x] domain: tipos + Ports `PlayerDataReader`/`CloudSaveReader`; erros `Err*Unavailable`
- [x] adapters `internal/playerdata` + `internal/cloudsave` (+ httptest)
- [x] service: deps + métodos `PlayerData`/`CloudSave` (struct `Deps` adotado — 7 deps)
- [x] transport: rotas `/data` e `/save` + `writeUpstream` (502; 404 repassa) + testes
- [x] main: `PLAYER_DATA_URL`/`CLOUD_SAVE_URL` + clients; compose

## admin-ui
- [x] `lib/api.ts`: `playerData` + `cloudSave`
- [x] `pages/PlayerDataView.tsx` + `pages/CloudSaveView.tsx`; `PlayerDetail` usa nas abas
- [x] `npm run build`

## verificação
- [x] `go test ./... -race` + build; Docker: abrir Players/(jogador)/Player data e Cloud saves
