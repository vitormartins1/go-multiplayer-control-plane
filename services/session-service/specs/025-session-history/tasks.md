# Tasks: 025-session-history

> Status: NADA implementado — spec Proposed aguardando priorização.

## Fatia 1 — domínio + store (session-service)
- [ ] `domain.Participant` + `Session.Participants` acumulado em `OpenWorld`/`JoinWorld`/`Create` (dedupe na re-entrada; papel da primeira ocorrência)
- [ ] Migrations `closed_sessions` + `closed_session_players` (schema do plan D4)
- [ ] Adapter Postgres `HistoryStore` (SaveClosed/ClosedByGame/ClosedByPlayer) + testes de integração
- [ ] `closeSession` grava no store APÓS o release (best-effort, log + métrica `session_history_persist_failures_total`)
- [ ] Compose: `session-postgres` + exporter + envs (gotcha: volume novo pras migrations)

## Fatia 2 — endpoints admin (session-service)
- [ ] `GET /admin/games/{gameId}/sessions?state=closed&limit=&before=` lendo o store (paginação por closed_at+session_id)
- [ ] `GET /admin/players/{playerId}/sessions?gameId=&limit=&before=`
- [ ] Fallback: Postgres fora → 503 `history_unavailable`; testes de handler

## Fatia 3 — admin-bff
- [ ] Proxies dos dois endpoints + enriquecimento de handles (padrão das salas)
- [ ] Spec-irmã curta em `services/admin-bff/specs/`

## Fatia 4 — admin-ui
- [ ] Aba **History** no PlayerDetail (após Achievements, escopo = jogo selecionado): closedAt, modo, papel, closeReason, duração, participantes
- [ ] Aba Sessions do jogo: fechadas do endpoint persistido + "carregar mais"
- [ ] Spec-irmã curta em `services/admin-ui/specs/`

## Validação final
- [ ] Bateria manual: mundo host_world com coop morto → histórico mostra o coop com role=coop (critério 4)
- [ ] Restart do session-service → fechadas persistem (critério 2)
