# Plan: 010-matchmaking-funnel

**Spec:** specs/010-matchmaking-funnel/spec.md
**Data:** 2026-07-04

## Abordagem técnica

UI-only. Liga a rota `/api/metrics/live` (já existente) num bloco de cards no Infrastructure. Extrai o card de estatística que hoje é privado da FanoutCards (009) para um `StatCard` reusável, e o usa nos dois blocos (fan-out + matchmaking). Um único `setInterval` refresca os dois sinais "ao vivo".

## Arquivos

- `src/lib/api.ts` — `interface LiveMetrics { queueSize, activeSessions, matchesTotal, matchesFailed: number | null }` + `api.liveMetrics(signal?)`.
- `src/pages/Overview.tsx`:
  - `const live = useAsync((s) => api.liveMetrics(s), [])`.
  - `setInterval` (~10s) chamando `live.reload()` + `fanout.reload()`; botão Atualizar também.
  - Bloco **Matchmaking** (4 `StatCard`) acima ou junto do bloco Fan-out.
  - Extrai `StatCard` (label + valor + cor opcional + destaque quando "ruim") — reusado por fan-out e matchmaking.

## Decisões

- **Reuso via `StatCard`** em vez de duplicar o markup dos cards (a FanoutCards vira consumidora do mesmo componente).
- **Polling 10s** (contadores operacionais; não precisa dos 5s do board de salas).
- **`matchesFailed`** e **consumos com erro** destacados em vermelho só quando `> 0`.

## Ordem

1. `api.ts` (tipo + método). 2. `Overview.tsx` (StatCard + bloco Matchmaking + polling). 3. `npm run build` + verificação visual.

## Riscos

- **Polling e re-render**: `useAsync` já lida com abort; dois reloads no mesmo intervalo é barato (2 GETs pequenos).
