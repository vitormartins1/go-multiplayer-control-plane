# Spec: 006-player-tabs

**Serviço:** admin-bff (+ admin-ui)
**Status:** Draft (preparada — depende de player-data/002 e cloud-save/002)
**Data:** 2026-06-30

## Contexto

Acende as abas **Player data** e **Cloud saves** do detalhe do jogador no painel (hoje "em breve"), espelhando o que fizemos para Leaderboards (`005-leaderboard-panel`). Depende dos endpoints admin read criados em `player-data/002-game-scope` e `cloud-save/002-game-scope`. O BFF já injeta a internal key (ADR-0018).

## Comportamento

- WHEN `GET /api/games/{gameId}/players/{playerID}/data` (admin-bff) -> lista `{namespace, key, value}` do jogador no jogo (lê o player-data).
- WHEN `GET /api/games/{gameId}/players/{playerID}/save` (admin-bff) -> save do jogador no jogo (lê o cloud-save); `404` repassado se não houver.
- IF o upstream estiver fora -> `502` (`*_unavailable`), como os outros.
- THE admin-ui SHALL trocar os `ComingSoon` das abas **Player data** e **Cloud saves** (em `PlayerDetail`) por tabelas/visões reais, com seletor de jogo (como em `PlayerAchievements`), carregamento on-open + Atualizar.

## Contratos de Interface

- admin-bff: `GET /api/games/{gameId}/players/{playerID}/data`, `GET /api/games/{gameId}/players/{playerID}/save`
- Ports `PlayerDataReader.ForPlayer(...)`, `CloudSaveReader.ForPlayer(...)`; adapters `internal/playerdata`, `internal/cloudsave` (espelham `internal/leaderboard`); erros `ErrPlayerDataUnavailable`, `ErrCloudSaveUnavailable` -> 502
- admin-ui: `api.playerData(gameId, playerID)`, `api.cloudSave(gameId, playerID)`; componentes `PlayerDataView`, `CloudSaveView` nas abas (com seletor de jogo)

## Casos de Teste Obrigatórios
- [ ] (bff) ambos os endpoints 200 + 502 quando upstream fora
- [ ] (bff) `/save` repassa 404 quando não há save
- [ ] (ui) abas listam dados; estados vazio/erro; build ok

## Fora do escopo
- Edição pelo painel (read-only)
- `URL`s `PLAYER_DATA_URL`/`CLOUD_SAVE_URL` no compose do admin-bff (adicionar junto)
