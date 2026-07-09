# Spec: 020-host-world-console

**Serviço:** admin-ui
**Status:** Done (2026-07-07)
**Data:** 2026-07-07
**ADR:** 0023 — **arquétipo `host_world_dropin`, fatia H3** (surfacing — lado UI)
**Par de:** `session-service/014`, `admin-bff/014`
**Depende de:** `admin-bff/014` (Rooms com origin/host/roleSlots; endpoint per-game analytics com roleParticipations).

## Contexto

Fecha a H3 no painel: o console passa a **mostrar o modelo host-cêntrico** — mundos abertos com papéis/slots (co-op X/2, invasor Y/1) em vez de "N players", e as participações por papel no detalhe de analytics do jogo.

## Comportamento

### Rooms — mundos host_world

- WHEN uma sala é `origin="host_world"` → THE SYSTEM SHALL mostrar, na coluna Players, os **slots por papel** (`RoleSlots`: co-op/invasor com `occupied/capacity`, papel cheio destacado) em vez da lista de avatares. `queue_arena` (`origin="match"`) segue mostrando os avatares.
- WHEN a sala host_world é expandida → THE SYSTEM SHALL mostrar o **host** (`hostPlayerId`), o rótulo "MUNDO" (em vez de "MATCH ID") e a quebra de slots por papel.

### Analytics por jogo — participações por papel

- THE SYSTEM SHALL trocar o `GameAnalytics` do painel para o endpoint **por jogo** (`api.gameAnalytics(gameId)`, em vez de filtrar o overview no cliente) — assim tem acesso a `roleParticipations`.
- WHEN o jogo tem `roleParticipations` (host_world_dropin com eventos reportados) → THE SYSTEM SHALL mostrar uma **tabela de participações por papel/evento** (papel, evento com rótulo amigável, total). Jogo sem participações (queue_arena) → a tabela não aparece; o resumo win/loss/draw segue igual.

## Contratos de Interface

- **`lib/api.ts`:** `GameSession` += `origin?`, `hostPlayerId?`, `roleSlots?: RoleSlot[]`; novos `RoleSlot`, `ParticipationStat`, `GameAnalyticsDetail`; `api.gameAnalytics(gameId)`.
- **`pages/Rooms.tsx`:** componente `RoleSlots` (badges por papel) + render condicional por `origin`.
- **`pages/GameAnalytics.tsx`:** usa `api.gameAnalytics`; tabela de `roleParticipations` (rótulos pt-BR de papel/evento).

## Fora de escopo

- Quadro de sinais ativo por mundo — follow-up.
- Gráfico/série temporal de participações — futuro (a tabela basta para a H3).

## Critérios de Aceite

- [ ] host_world no Rooms: slots por papel (co-op X/2, invasor Y/1), host no detalhe; queue_arena inalterado.
- [ ] `GameAnalytics` usa o endpoint per-game; participações por papel aparecem quando há; ausentes no queue_arena.
- [ ] `tsc --noEmit` + `vite build` verdes.
