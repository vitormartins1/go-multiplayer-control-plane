# admin-ui

Front-end do **painel de administração** ("sala de controle" — ADR-0016). SPA **React + Vite + TypeScript** organizada **por domínio** (espelhando os serviços Go). Consome a API do `admin-bff`. **Read-only**, sem auth no MVP.

> App de apresentação — fora do hexagonal Go. Segue um **SDD adaptado** (spec = telas/UX): ver `specs/001-control-room/`.

## Rodar (dev)

```bash
cd services/admin-ui
npm install
npm run dev        # http://localhost:5173
```

Precisa do `admin-bff` no ar (porta 8088). O Vite faz **proxy** de `/api` -> `http://localhost:8088` (sem CORS). Para apontar a outra porta/host:

```bash
VITE_BFF_URL=http://localhost:9999 npm run dev
# ou copie .env.example para .env e ajuste
```

Subir o BFF + fontes localmente: `docker compose -f infra/docker/docker-compose.yml up admin-bff prometheus achievements-service ...` (ou a stack toda).

## Telas (sidebar por domínio)

- **Visão geral** — topologia (React Flow, read-only) dos serviços com cor por status (verde/vermelho/cinza) + contadores ao vivo. Clicar num nó navega ao domínio.
- **Por domínio** (User base, Achievements, Matchmaking, Sessions, Player data, Cloud save, Leaderboard) — saúde + **RED** (req/s, erro %, latência p95) + runtime; contadores específicos (matchmaking/sessions); bloco **"Iteração 2"** com o que ainda falta naquele domínio.
- **Achievements** — além do padrão, busca por `playerID` (id da conta / sub, não o handle).
- **Game analytics** — *em breve* (o serviço ainda não existe).

**Auto-refresh:** toggle + intervalo (5/15/30s), default ligado @ 5s, aplicado aos dados ao vivo (atualização suave, sem piscar). Achievements é sob demanda.

## O que consome (admin-bff)

`/api/topology`, `/api/metrics/live`, `/api/services/{id}/metrics`, `/api/players/{playerID}/achievements`.

## Estrutura

```
src/
  main.tsx, App.tsx, index.css   # entrypoint + tema dark
  lib/      api.ts (fetch+tipos), domains.ts (catálogo por domínio), useLive.ts (polling)
  components/  Sidebar, RefreshControls, Overview, TopologyGraph, DomainPage,
               AchievementsPanel, MetricCard, StatusBadge, Loading, ErrorState
```

As páginas de domínio são dirigidas por dados (`lib/domains.ts`): adicionar/ajustar um domínio é editar esse catálogo, não criar telas.

## Build

```bash
npm run build      # type-check (tsc) + bundle (vite) -> dist/
```
