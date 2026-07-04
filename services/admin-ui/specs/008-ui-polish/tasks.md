# Tasks: 008-ui-polish

## Implementação

- [x] `App.tsx` — renomear "Visão geral" → "Infrastructure", mover para último, rota `/infrastructure`, redirect `/` → `/games`
- [x] `Overview.tsx` — atualizar título para "Infrastructure"
- [x] `GameDetail.tsx` — buscar contagens (catalog, players, sessions) e exibir `CountBadge` nas abas
- [x] `AchievementIcon.tsx` — criar componente com DiceBear glyphs via API pública
- [x] `GameAchievements.tsx` — adicionar coluna de ícone na tabela
- [x] `GameSessions.tsx` — `expandedId` state + `SessionRow` expansível com painel de detalhes
- [x] Build de verificação (`tsc --noEmit` + `vite build`) — passou sem erros
- [x] Criar artefatos SDD (spec, plan, tasks)
