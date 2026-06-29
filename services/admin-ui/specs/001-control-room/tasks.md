# Tasks: 001-control-room
**Plan:** specs/001-control-room/plan.md
**Data:** 2026-06-29

> SPA React+Vite+TS (fora do hexagonal Go). Consome o admin-bff via proxy do Vite.
> Depende de admin-bff/002 (páginas de domínio). Marcar [x] conforme implementa.

## Fase 1 — Scaffold
- [ ] `package.json`, `tsconfig.json`, `tsconfig.node.json`, `vite.config.ts` (proxy `/api`->:8088), `index.html`, `.gitignore`, `.env.example`
- [ ] `src/main.tsx`, `src/App.tsx` (layout base: sidebar + conteúdo), `src/index.css` (tema dark + variáveis de status)

## Fase 2 — Lib (API + polling)
- [ ] `src/lib/api.ts` — tipos espelhando o BFF + chamadas (`topology`, `liveMetrics`, `serviceMetrics`, `playerAchievements`)
- [ ] `src/lib/domains.ts` — catálogo de domínios (id, label, counters específicos, itens "iteração 2", flag "em breve")
- [ ] `src/lib/useLive.ts` — hook de polling (toggle + intervalo 5/15/30s) com atualização suave (mantém último dado bom; AbortController)

## Fase 3 — Navegação e layout
- [ ] `src/components/Sidebar.tsx` — navegação por domínio; seção ativa destacada; "em breve" desabilitado
- [ ] `src/components/RefreshControls.tsx` — toggle auto-refresh + seletor de intervalo
- [ ] `src/components/Loading.tsx`, `src/components/ErrorState.tsx` (com "tentar de novo"), `src/components/StatusBadge.tsx`, `src/components/MetricCard.tsx`

## Fase 4 — Visão geral (topologia)
- [ ] `src/components/TopologyGraph.tsx` — React Flow (`@xyflow/react`): nós coloridos por status, read-only (sem drag/edição), pan/zoom, clique no nó -> domínio
- [ ] `src/components/Overview.tsx` — topologia + contadores globais (`/api/metrics/live`); banner quando `sourceAvailable=false`; `null` -> "—"

## Fase 5 — Páginas de domínio
- [ ] `src/components/DomainPage.tsx` — genérico: saúde + RED (`/api/services/{id}/metrics`); cards específicos (matchmaking/sessions via live); bloco "Iteração 2"; estado degradado se a fonte cair
- [ ] Seção **Game analytics** "em breve" (sem chamadas)

## Fase 6 — Achievements
- [ ] `src/components/AchievementsPanel.tsx` — busca por `playerID` (hint: id da conta, não handle); lista progresso/estado; vazio -> mensagem; 502 -> erro

## Fase 7 — Docs e verificação
- [ ] `README.md` — rodar (`npm install`, `npm run dev`, `.env`/proxy), o que consome, escopo do MVP
- [ ] Verificação: `npm run build` (type-check + bundle) com BFF no ar; navegar as seções; auto-refresh; degradação
