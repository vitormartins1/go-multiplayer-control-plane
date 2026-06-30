# Tasks: 005-game-library
**Plan:** specs/005-game-library/plan.md
**Data:** 2026-06-30

- [x] `components/GameCover.tsx` — capa procedural 9:16 (cover + mini)
- [x] `lib/games.tsx` — GameProvider + useGames (lista + jogo ativo em localStorage)
- [x] `main.tsx` — envolve App com GameProvider
- [x] `App.tsx` — seletor de jogo ativo no header
- [x] `pages/Games.tsx` — biblioteca de capas (SimpleGrid); clica = ativo + detalhe
- [x] `pages/PlayerDetail.tsx` — passa gameId do contexto às abas
- [x] `PlayerAchievements`/`PlayerDataView`/`CloudSaveView` — gameId por prop; sem Select
- [x] `npm run build` + validar no Docker (rebuild admin-ui)
