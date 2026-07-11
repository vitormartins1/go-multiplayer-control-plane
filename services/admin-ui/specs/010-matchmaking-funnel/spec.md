# Spec: 010-matchmaking-funnel

**Serviço:** admin-ui
**Status:** Done (2026-07-04)
**Data:** 2026-07-04

## Contexto

O admin-bff já expõe `GET /api/metrics/live` (`LiveMetrics`: `queueSize`, `activeSessions`, `matchesTotal`, `matchesFailed`), mas o **admin-ui nunca consome** essa rota — o funil do matchmaking está invisível no painel. Quick win: ligar esses contadores no Infrastructure, dando visibilidade ao começo do gameplay loop (fila → match → sessão). Zero backend novo (a rota e as métricas do matchmaking já existem).

## Comportamento

- THE SYSTEM SHALL adicionar um bloco **"Matchmaking"** no Infrastructure com cards de: **Fila** (`queueSize`), **Sessões ativas** (`activeSessions`), **Matches formados** (`matchesTotal`) e **Matches com falha** (`matchesFailed`, vermelho quando > 0).
- THE SYSTEM SHALL tratar `null` (métrica ausente / Prometheus fora) como **"—"** (degrada, não quebra), igual aos cards de fan-out (009).
- THE SYSTEM SHALL atualizar os contadores ao vivo: no botão **Atualizar** existente e por **polling** curto (junto com a saúde do fan-out — são sinais operacionais "ao vivo").

## Contratos de Interface (cliente)

- `src/lib/api.ts`: tipo `LiveMetrics` (campos `number | null`) + `api.liveMetrics()` → `GET /api/metrics/live`.
- `src/pages/Overview.tsx`: bloco Matchmaking; reuso do mesmo componente de card dos sinais de fan-out (extrair `StatCard` para não duplicar markup).

## Casos de Teste / Verificação

- [ ] Infrastructure mostra os 4 cards do matchmaking com os valores do `/api/metrics/live`
- [ ] `matchesFailed > 0` fica em vermelho; `null` vira "—"
- [ ] Atualizar/polling refletem novos valores; Prometheus fora → cards degradam sem quebrar
- [ ] `npm run build` (typecheck) limpo

## Fora do escopo

- **Tendência temporal** (séries no tempo) — fica no Grafana; aqui é card instantâneo.
- Lista de **tickets na fila** (quem está esperando) — precisaria de endpoint novo no matchmaking; futuro.
