# Tasks: 006-player-tabs
**Plan:** specs/006-player-tabs/plan.md
**Data:** 2026-06-30
**Pré-requisito:** player-data/002-game-scope + cloud-save/002-game-scope

## admin-bff
- [ ] domain: tipos + Ports `PlayerDataReader`/`CloudSaveReader`; erros `Err*Unavailable`
- [ ] adapters `internal/playerdata` + `internal/cloudsave` (+ httptest)
- [ ] service: deps + métodos `PlayerData`/`CloudSave` (avaliar struct `Deps`)
- [ ] transport: rotas `/data` e `/save` + `writeUpstream` (502; 404 repassa) + testes
- [ ] main: `PLAYER_DATA_URL`/`CLOUD_SAVE_URL` + clients; compose

## admin-ui
- [ ] `lib/api.ts`: `playerData` + `cloudSave`
- [ ] `pages/PlayerDataView.tsx` + `pages/CloudSaveView.tsx`; `PlayerDetail` usa nas abas
- [ ] `npm run build`

## verificação
- [ ] `go test ./... -race` + build; Docker: abrir Players/(jogador)/Player data e Cloud saves
