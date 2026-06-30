# Plan: 006-player-tabs
**Spec:** specs/006-player-tabs/spec.md
**Data:** 2026-06-30
**Depende de:** player-data/002-game-scope, cloud-save/002-game-scope (endpoints admin read)

## admin-bff (mesmo padrão de internal/leaderboard de 005)
- `internal/domain/admin.go` — tipos `PlayerDataEntry{Namespace,Key,Value}`, `CloudSave{Size,UpdatedAt,Content?}`; Ports `PlayerDataReader.ForPlayer(ctx,gameID,playerID)`, `CloudSaveReader.ForPlayer(ctx,gameID,playerID)`
- `internal/domain/errors.go` — `ErrPlayerDataUnavailable`, `ErrCloudSaveUnavailable`
- `internal/playerdata/client.go` + `internal/cloudsave/client.go` (+ httptest) — adapters HTTP (do() classifica; 404 -> UpstreamError 404; conn/5xx -> Err*Unavailable)
- `internal/service/admin_service.go` — campos + `New(...)` (já vai ficar grande; considerar agrupar deps num struct `Deps` se preferir) + métodos `PlayerData`, `CloudSave`
- `internal/transport/http_handler.go` — interface + rotas `GET /api/games/{gameId}/players/{playerID}/data` e `.../save`; `writeUpstream` cobre os 2 novos erros (502); 404 repassa
- `cmd/server/main.go` — `PLAYER_DATA_URL`, `CLOUD_SAVE_URL` + clients; compose
- testes service/transport: fakes ganham os métodos; 200/502/404

## admin-ui
- `src/lib/api.ts` — tipos + `api.playerData`, `api.cloudSave`
- `src/pages/PlayerDataView.tsx`, `src/pages/CloudSaveView.tsx` — seletor de jogo (como `PlayerAchievements`) + tabela/visão + Atualizar
- `src/pages/PlayerDetail.tsx` — abas Player data / Cloud saves usam os componentes (eram ComingSoon)
- `npm run build`

## Decisões
- **Nota sobre `service.New`**: já recebe 5 deps; +2 = 7. Avaliar refatorar para `service.New(Deps{...})` (struct) nesta feature — reduz churn futuro. Decidir no início.
- Read-only; seletor de jogo nas abas (jogador é global, dado é por jogo).

## Ordem
1. (pré-requisito) implementar player-data/002 e cloud-save/002
2. bff: domain -> adapters+test -> service -> transport -> main/compose
3. ui: api -> views -> PlayerDetail -> build
4. Docker: gerar dado (via API player com X-Game-Id) -> abrir Players/(jogador)/Player data e Cloud saves

## Riscos
- **service.New crescendo**: bom momento para o struct `Deps` (decisão no item acima).
- **Conteúdo do save** (binário): definir shape do JSON (base64?) alinhado ao admin read do cloud-save/002.
