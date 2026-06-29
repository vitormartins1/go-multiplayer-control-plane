# Tasks: 004-redesign
**Plan:** specs/004-redesign/plan.md
**Data:** 2026-06-29

> Mantine + react-router; IA por entidade (Games/Players); saúde leve; sem auto-refresh.

## Fase 1 — Setup
- [ ] deps (Mantine, @tabler/icons-react, react-router-dom, postcss-preset-mantine, postcss-simple-vars); `postcss.config.cjs`
- [ ] `src/main.tsx` — MantineProvider (dark) + Notifications + BrowserRouter; enxugar `index.css`
- [ ] `src/App.tsx` — AppShell (navbar 4 seções) + Routes

## Fase 2 — Lib
- [ ] `src/lib/api.ts` — remover serviceMetrics/liveMetrics + tipos RED

## Fase 3 — Páginas
- [ ] `Overview` (mapa + saúde leve + Atualizar); `ComingSoon`; `Analytics`
- [ ] `Games` (lista + modal registrar); `GameDetail` (tabs); `GameAchievements` (catálogo + modal)
- [ ] `Players` (busca + tabela paginada); `PlayerDetail` (tabs); `PlayerAchievements` (seletor jogo + tabela)

## Fase 4 — Limpeza
- [ ] remover componentes/arquivos antigos (Sidebar/RefreshControls/DomainPage/GamesPanel/AchievementsPanel/UsersPanel/MetricCard/useLive/domains)
- [ ] `npm run build` (tsc + vite) OK

## Fase 5 — Companion admin-bff (remover RED)
- [ ] remover `/api/services/{id}/metrics` (service/transport/domain ServiceMetrics, ErrUnknownService) + testes
- [ ] `go test ./... -race` + `go build ./...`

## Fase 6 — Verificação
- [ ] rebuild imagens; Docker: navegar seções, registrar jogo + conquista, players; saúde no mapa
