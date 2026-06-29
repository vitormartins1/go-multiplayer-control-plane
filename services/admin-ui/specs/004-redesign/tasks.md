# Tasks: 004-redesign
**Plan:** specs/004-redesign/plan.md
**Data:** 2026-06-29

> Mantine + react-router; IA por entidade (Games/Players); saúde leve; sem auto-refresh.

## Fase 1 — Setup
- [x] deps (Mantine, @tabler/icons-react, react-router-dom, postcss-preset-mantine, postcss-simple-vars); `postcss.config.cjs`
- [x] `src/main.tsx` — MantineProvider (dark) + Notifications + BrowserRouter; enxugar `index.css`
- [x] `src/App.tsx` — AppShell (navbar 4 seções) + Routes

## Fase 2 — Lib
- [x] `src/lib/api.ts` — remover serviceMetrics/liveMetrics + tipos RED

## Fase 3 — Páginas
- [x] `Overview` (mapa + saúde leve + Atualizar); `ComingSoon`; `Analytics`
- [x] `Games` (lista + modal registrar); `GameDetail` (tabs); `GameAchievements` (catálogo + modal)
- [x] `Players` (busca + tabela paginada); `PlayerDetail` (tabs); `PlayerAchievements` (seletor jogo + tabela)

## Fase 4 — Limpeza
- [x] remover componentes/arquivos antigos (Sidebar/RefreshControls/DomainPage/GamesPanel/AchievementsPanel/UsersPanel/MetricCard/useLive/domains)
- [x] `npm run build` (tsc + vite) OK

## Fase 5 — Companion admin-bff (remover RED)
- [x] remover `/api/services/{id}/metrics` (service/transport/domain ServiceMetrics, ErrUnknownService) + testes
- [x] `go test ./... -race` + `go build ./...`

## Fase 6 — Verificação
- [x] rebuild imagens; Docker: navegar seções, registrar jogo + conquista, players; saúde no mapa
