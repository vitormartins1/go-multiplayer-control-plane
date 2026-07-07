# Spec: 017-board-selector

**Serviço:** admin-ui
**Status:** Draft
**Data:** 2026-07-06
**Parte de:** B2b do roadmap `queue_arena` (ADR-0023). Par: `admin-bff/013-board-passthrough`.

## Contexto

O leaderboard-service tem N rankings por jogo (`score` + um por stat declarado, leaderboard/003). A aba **Leaderboards** só mostrava o board `score`. Esta fatia adiciona um **seletor de board**, com as opções derivadas do **perfil de multiplayer** do jogo (B1): `score` + `scoringStats`.

## Comportamento

- THE SYSTEM SHALL, na aba Leaderboards, carregar o perfil (`getMultiplayerProfile`) e montar as opções de board = `["score", ...profile.scoringStats]`. Jogo sem perfil (ou sem stats) → só `score`; o seletor só aparece quando há mais de um board.
- WHEN o board muda → THE SYSTEM SHALL recarregar o top-N daquele board (`gameLeaderboard(gameId, board)`); `score` é enviado como board omitido (default do serviço).
- THE SYSTEM SHALL rotular a coluna de valor e o override de score conforme o board selecionado (o "score" de um board de stat é o total daquela métrica).
- O override de score (modal) SHALL aplicar no board selecionado (`setLeaderboardScore(..., board)`).

## Contratos de Interface (cliente)

- `src/lib/api.ts`: `gameLeaderboard(gameId, board?, limit?, signal?)` e `setLeaderboardScore(gameId, playerId, score, board?)` ganham `board` opcional (query param; omitido → score).
- `src/pages/GameLeaderboards.tsx`: estado `board`; `Select` de board no header (só se >1); `useAsync` depende de `[gameId, board]`; header da coluna + override refletem o board.

## Casos de Teste / Verificação

- [ ] jogo com `scoringStats` → seletor lista `Score` + os stats; trocar recarrega o ranking do board
- [ ] jogo sem perfil → seletor oculto, só `score`
- [ ] override de score num board de stat vai para aquele board
- [ ] `npm run build` (typecheck) limpo

## Fora do escopo

- Boards acumulativos / por outcome (A2); paginação/seasons do leaderboard (backlog).
