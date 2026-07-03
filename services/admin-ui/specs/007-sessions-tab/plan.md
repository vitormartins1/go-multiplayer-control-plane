# Plan: 007-sessions-tab

**Spec:** [spec.md](spec.md)
**Data:** 2026-07-03

## 1. API client (`lib/api.ts`)

- Tipos `GameSession`, `SessionPlayer`, `GameSessions {active, closed}` + `fetchGameSessions(gameId, state?)` — padrão dos fetches existentes.

## 2. Página (`pages/GameSessions.tsx`)

- Base no `GameLeaderboards.tsx`/`GamePlayers.tsx` (tabela + estados de loading/erro/empty da casa, hook `useAsync`).
- Polling: `setInterval` 5s disparando o refetch **só com a aba montada** (Tabs desmonta painel? conferir `keepMounted` do Mantine — se mantém montado, gate por visibilidade da aba); cleanup no unmount.
- Duração/“há Xmin”: helper local (`Intl`/date-fns se a casa já usa — conferir; senão diff manual).
- Badges: `open` → `color="green"`, `closed` → `gray` com motivo.

## 3. GameDetail

- `Tabs.Tab value="sessions"` (ex.: `IconServer2`/`IconBroadcast` dos @tabler já usados) + `Tabs.Panel` com `GameSessions`.

## Riscos / notas

- Ordem: BFF 007 primeiro (contrato).
- Se o enriquecimento de handle não sair no BFF, mostrar `id` truncado (`guest-1fa3…`) — sem bloquear.
- Vite proxy já cobre `/api` → nada de config nova.
