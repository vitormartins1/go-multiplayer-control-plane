# Plan: 005-game-library
**Spec:** specs/005-game-library/spec.md
**Data:** 2026-06-30

## Abordagem
UI pura (sem backend). Contexto de jogo global via React Context + localStorage;
biblioteca de capas com capa procedural; abas do jogador passam a receber o
`gameId` do contexto (deixam de ter `<Select>` próprio).

## Arquivos
- `src/components/GameCover.tsx` — capa procedural 9:16 (hash do slug → hue;
  gradiente + inicial + nome). Prop `variant: "cover" | "mini"`.
- `src/lib/games.tsx` — `GameProvider` (carrega `api.games`, guarda `activeGameId`
  em localStorage) + hook `useGames()` (`{ games, activeGame, activeGameId,
  setActiveGameId, loading, error, reload }`). Fonte única da lista de jogos.
- `src/main.tsx` — envolve `<App/>` com `<GameProvider>`.
- `src/App.tsx` — header ganha o seletor de jogo ativo (Menu com capa-mini + nome).
- `src/pages/Games.tsx` — tabela → `SimpleGrid` de `GameCover` (clica = seleciona
  ativo + `nav(/games/:id)`); mantém Registrar/Atualizar; usa `useGames()`.
- `src/pages/PlayerDetail.tsx` — lê `activeGame` do contexto; passa `gameId` às abas;
  subtítulo com o jogo ativo.
- `src/pages/PlayerAchievements.tsx`, `PlayerDataView.tsx`, `CloudSaveView.tsx` —
  assinatura passa a `{ playerId, gameId }`; removem o `<Select>` e o fetch de
  `api.games`; sem `gameId` → prompt "selecione um jogo no topo".

## Decisões
- **Contexto único de jogos**: o `GameProvider` é a fonte da lista (evita N fetches
  de `api.games` espalhados). Games e header consomem o mesmo.
- **Capa procedural determinística** (mesmo slug → mesma cor) para estabilidade visual.
- **Views recebem `gameId` por prop** (não leem o contexto direto) — ficam puras e
  reaproveitáveis; o PlayerDetail injeta o do contexto.
- `GameDetail` (rota `/games/:id`) permanece via `useParams` (não depende do ativo).

## Ordem
1. GameCover → games.tsx (provider/hook) → main wire
2. App header switcher → Games biblioteca
3. PlayerDetail + 3 views (gameId via prop)
4. `npm run build`; validar no Docker (rebuild admin-ui)

## Riscos
- `activeGameId` em localStorage pode apontar para jogo removido → `activeGame`
  resolve para null (trata como "sem ativo").
- StrictMode duplica efeitos em dev — o provider usa `useAsync` já existente (ok).
