# Plan: 003-games
**Spec:** specs/003-games/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Increment do front (React+Vite+TS). Adiciona a seção **Games** (novo `kind` no catálogo de domínios) renderizada por um `GamesPanel`, e um estado **`selectedGame`** elevado ao `App` (prop drilling, sem state lib — consistente com 001/002). O `GamesPanel` é o hub: registrar/listar/selecionar jogo + catálogo + form de conquista. O `AchievementsPanel` passa a ser **game-scoped** (recebe `selectedGame`), consertando a chamada antiga. Mensagens de validação (400/409) vêm do `HttpError.status` repassado pelo BFF.

## Arquivos a criar ou modificar

- `src/lib/api.ts` — tipos `Game`, `CatalogAchievement`; funções `games()`, `createGame(slug,name)`, `gameCatalog(gameId)`, `createAchievement(gameId, body)`; mudar `playerAchievements(playerID)` -> `playerAchievements(gameId, playerID)` (rota nova)
- `src/lib/domains.ts` — entrada `{ id:"games", label:"Games", kind:"games" }` após "overview"; estender o union de `kind` com `"games"`
- `src/App.tsx` — estado `selectedGame: Game | null`; renderiza `GamesPanel` quando `kind==="games"`; passa `selectedGame` ao `DomainPage`
- `src/components/GamesPanel.tsx` (novo) — lista de jogos (selecionável) + form registrar (slug/name); jogo selecionado mostra catálogo (`gameCatalog`) + form adicionar conquista; trata 400/409/502
- `src/components/AchievementsPanel.tsx` — recebe `selectedGame`; busca `playerAchievements(gameId, playerID)`; se não há jogo, orienta a selecionar
- `src/components/DomainPage.tsx` — recebe e repassa `selectedGame` ao `AchievementsPanel`
- `src/components/Sidebar.tsx` — sem mudança estrutural (Games é só mais uma entrada navegável)

## Decisões técnicas e trade-offs
- **`selectedGame` no `App` (prop drilling)**: estado compartilhado simples; sem context/Redux (mesma linha do 001/002). O `GamesPanel` escreve, o `AchievementsPanel` lê.
- **Games como `kind` próprio** (não "service"): não tem RED/topologia; é gestão de tenant. O `App` despacha para o `GamesPanel`.
- **Mensagens de erro pelo status**: o BFF repassa 400/409 (UpstreamError); o front mapeia `HttpError.status` para texto ("slug já existe", "id já existe neste jogo").
- **Seleção via lista** (clicar no jogo) em vez de dropdown global no header: menos UI, e a seção Games já é o lugar natural de escolher; um dropdown por página fica como evolução.

## Ordem de implementação
1. `lib/api.ts` (tipos + funções de jogos; ajustar `playerAchievements`)
2. `lib/domains.ts` (entrada Games + kind)
3. `App.tsx` (estado `selectedGame` + roteia GamesPanel + passa ao DomainPage)
4. `GamesPanel.tsx` (registrar/listar/selecionar + catálogo + add conquista)
5. `AchievementsPanel.tsx` (game-scoped) + `DomainPage.tsx` (repassa selectedGame)
6. `npm run build` (tsc + vite); rebuild da imagem; validar no Docker (registrar jogo, cadastrar conquista, ver catálogo, repasse de 409)

## Riscos e pontos de atenção
- **Tipos compartilhados com o BFF**: `Game`/`CatalogAchievement` espelham o JSON do BFF (`createdAt`, `target` etc.).
- **`createAchievement` precisa de `eventType`**: o form coleta id/name/description/eventType/target; o BFF/achievements exige eventType.
- **Sem jogo selecionado**: o `AchievementsPanel` não deve chamar a API com gameId vazio — guardar e orientar.
- **Build sem browser**: type-check + bundle validáveis; navegação visual fica para verificação manual (como nas 001/002).
- **Estado de seleção ao registrar**: ao criar um jogo com sucesso, selecioná-lo automaticamente (UX do fluxo "registrar → cadastrar achievements").
