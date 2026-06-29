# Tasks: 003-games
**Plan:** specs/003-games/plan.md
**Data:** 2026-06-29

> Seção Games + achievements por jogo. Fecha o multi-tenant no painel.

- [x] `src/lib/api.ts` — tipos `Game`/`CatalogAchievement`; `games()`, `createGame`, `gameCatalog`, `createAchievement`; `playerAchievements(gameId, playerID)`
- [x] `src/lib/domains.ts` — entrada `Games` (kind `games`) após overview; estende o union de kind
- [x] `src/App.tsx` — estado `selectedGame`; roteia `GamesPanel` (kind games); passa `selectedGame` ao `DomainPage`
- [x] `src/components/GamesPanel.tsx` — registrar/listar/selecionar jogo + catálogo do selecionado + form adicionar conquista; trata 400/409/502
- [x] `src/components/AchievementsPanel.tsx` — game-scoped (recebe `selectedGame`; sem jogo -> orienta)
- [x] `src/components/DomainPage.tsx` — repassa `selectedGame` ao `AchievementsPanel`
- [x] `npm run build` OK; rebuild da imagem; validar no Docker (registrar jogo, cadastrar conquista, catálogo, repasse de 409)
