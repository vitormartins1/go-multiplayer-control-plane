# Plan: 004-redesign
**Spec:** specs/004-redesign/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Adota **Mantine 7** (`@mantine/core`, `@mantine/hooks`, `@mantine/form`, `@mantine/notifications`, `@tabler/icons-react`) + **react-router-dom 6**. O `AppShell` do Mantine substitui o layout/sidebar custom; o tema dark substitui o `index.css` próprio (mantém só ajustes pontuais). Rotas aninhadas (lista → detalhe → aba). Remove RED e auto-refresh.

## Arquivos a criar ou modificar

**Setup**
- `package.json` — add Mantine, `@tabler/icons-react`, `react-router-dom`, `postcss-preset-mantine`, `postcss-simple-vars`
- `postcss.config.cjs` — preset Mantine
- `src/main.tsx` — `MantineProvider` (dark) + `Notifications` + `BrowserRouter`
- `src/index.css` — enxugar (Mantine cuida do tema; manter só o nó do React Flow)

**Layout e rotas**
- `src/App.tsx` — `AppShell` (navbar com as 4 seções via `NavLink`) + `<Routes>`
- `src/lib/api.ts` — remover `serviceMetrics`/`liveMetrics`/tipos RED; manter games/players/achievements/topology

**Páginas (`src/pages/`)**
- `Overview.tsx` — mapa (reusa `TopologyGraph`) + saúde leve; botão Atualizar
- `Games.tsx` — `Table` de jogos + `Modal` registrar (`@mantine/form`)
- `GameDetail.tsx` — header + `Tabs` (Achievements | Leaderboards | Configurações)
- `GameAchievements.tsx` — catálogo (`Table`) + `Modal` nova conquista
- `Players.tsx` — busca + `Table` paginada (reusa lógica do UsersPanel)
- `PlayerDetail.tsx` — header + `Tabs` (Achievements | Player data | Cloud saves)
- `PlayerAchievements.tsx` — seletor de jogo (`Select`) + tabela de progresso
- `ComingSoon.tsx` — placeholder reutilizável das abas/seções futuras
- `Analytics.tsx` — usa `ComingSoon`

**Remover** (substituídos): `components/Sidebar.tsx`, `RefreshControls.tsx`, `DomainPage.tsx`, `GamesPanel.tsx`, `AchievementsPanel.tsx`, `UsersPanel.tsx`, `MetricCard.tsx`, `lib/useLive.ts`, `lib/domains.ts` (vira rotas). `TopologyGraph.tsx`, `StatusBadge.tsx`, `ErrorState.tsx`, `Loading.tsx` ficam (adaptados ao Mantine quando fizer sentido).

**Companion (admin-bff)**
- Remover `/api/services/{id}/metrics`: `ServiceMetrics` do service/transport/`AdminService`, tipo `domain.ServiceMetrics`, `ErrUnknownService`, e os testes correspondentes. (Topologia já dá a saúde.)

## Decisões técnicas e trade-offs
- **Mantine** (vs shadcn/antd): batteries-included p/ back-office, dark nativo, setup leve no Vite. Trade-off: mais deps, mas elimina CSS custom e padroniza.
- **react-router**: a IA por entidade (detalhe + abas) pede URLs reais; back/refresh/deep-link funcionam. Custo: uma dep.
- **Saúde da topologia, não endpoint dedicado**: a Visão geral já busca `/api/topology` (tem `up` de todos) — o detalhe de serviço não precisa de chamada própria; por isso o RED do BFF sai.
- **Esqueletos "em breve"**: estrutura visível do roadmap sem backend — orienta as próximas iterações.

## Ordem de implementação
1. Setup (deps, postcss, MantineProvider + Router em main, App AppShell + rotas vazias)
2. `lib/api.ts` (limpar RED)
3. Overview (mapa + saúde) e `ComingSoon`/Analytics
4. Games (lista + modal) → GameDetail (tabs) → GameAchievements (catálogo + modal)
5. Players (lista) → PlayerDetail (tabs) → PlayerAchievements (seletor + tabela)
6. Remover arquivos antigos; `npm run build`
7. Companion admin-bff: remover RED; `go test`/`go build`
8. Rebuild imagens; validar no Docker (navegação, registrar jogo/conquista, players)

## Riscos e pontos de atenção
- **Mantine no Vite/Node 18**: fixar Mantine 7.x; postcss preset obrigatório (senão estilos quebram).
- **React Flow + Mantine**: o grafo tem CSS próprio; manter o import e um container; conferir tema escuro.
- **Churn grande**: muitos arquivos saem/entram; garantir build verde antes do commit.
- **BFF**: remover RED sem quebrar topologia/live (live pode sair também, mas o overview não usa mais — manter `/api/metrics/live` no BFF é inofensivo; só não consumimos).
- **Sem browser aqui**: validar `npm run build` + plumbing via curl; navegação visual é check manual local.
