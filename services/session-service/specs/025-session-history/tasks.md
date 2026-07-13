# Tasks: 025-session-history

> Status: Done (2026-07-13) — Fatias 1-4 implementadas e validadas ao vivo.

## Fatia 1 — domínio + store (session-service)
- [x] `domain.Participant` + `Session.Participants` acumulado em `OpenWorld`/`JoinWorld`/`Create` (dedupe na re-entrada; papel da ocorrência mais recente quando o papel muda)
- [x] Migrations `closed_sessions` + `closed_session_players` (schema do plan D4)
- [x] Adapter Postgres `HistoryStore` (SaveClosed/ClosedByGame/ClosedByPlayer) + testes de integração
- [x] `closeSession` grava no store APÓS o release (best-effort, log + métrica `session_history_persist_failures_total`)
- [x] Compose: `session-postgres` + exporter + envs (gotcha: volume novo pras migrations)

## Fatia 2 — endpoints admin (session-service)
- [x] `GET /admin/games/{gameId}/sessions?state=closed&limit=&before=` lendo o store (paginação por closed_at+session_id)
- [x] `GET /admin/players/{playerId}/sessions?gameId=&limit=&before=`
- [x] Fallback: Postgres fora → 503 `history_unavailable`; testes de handler

## Fatia 3 — admin-bff
- [x] Proxies dos dois endpoints + enriquecimento de handles (padrão das salas)
- [x] Spec-irmã curta em `services/admin-bff/specs/`

## Fatia 4 — admin-ui
- [x] Aba **History** no PlayerDetail (após Achievements, escopo = jogo selecionado): closedAt, modo, papel, closeReason, duração, participantes
- [x] Aba Sessions do jogo: fechadas do endpoint persistido + "carregar mais"
- [x] Spec-irmã curta em `services/admin-ui/specs/`

## Validação final
- [x] Bateria manual: mundo host_world com coop morto → histórico mostra o coop com role=coop (critério 4)
- [x] Restart do session-service → fechadas persistem (critério 2)
