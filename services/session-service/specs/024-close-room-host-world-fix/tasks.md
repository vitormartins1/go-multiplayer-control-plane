# Tasks: 024-close-room-host-world-fix
**Plan:** specs/024-close-room-host-world-fix/plan.md
**Data:** 2026-07-11

## Fase 1 — session-service
- [x] `resolveAdminCloseID(id)`: tenta `matchIndex[id]`, cai pra `sessions[id]`
- [x] Atualizar `CloseByAdmin` pra usar a resolução nova
- [x] Teste: force-close de sessão host_world_dropin (aberta via `OpenWorld`) por `sessionId` — sucesso (bug corrigido) — `TestCloseByAdmin_MundoHostWorld_FechaPorSessionID`
- [x] Teste: force-close de sessão queue_arena por `matchId` — continua funcionando (regressão) — já cobria (`TestCloseByAdmin_Ativa_FechaELiberaServer`), exercitado pela resolução nova
- [x] Teste: force-close de id desconhecido (nem `matchId` nem `sessionId`) — 404/`ErrSessionNotFound` — já cobria (`TestCloseByAdmin_Inexistente_ErrSessionNotFound`)

## Fase 2 — admin-ui
- [x] `Rooms.tsx`: `confirmClose` usa `closing.matchId || closing.sessionId`
- [x] `tsc --noEmit` + `npm run build` limpos

## Fase 3 — Verificação
- [x] `go test ./services/session-service/... -race` — 174 testes ok
- [x] Smoke ao vivo: abrir mundo host_world_dropin de teste, encerrar via admin-bff (`:8089`) usando `sessionId` — 200, `state=closed`, `closeReason=admin`, slot do allocator liberado (reutilizado por outra sessão em seguida)
- [x] Regressão ao vivo: force-close de sessão queue_arena por `matchId` — 200, continua funcionando
- [x] Confirmação do usuário — "validado." (2026-07-11)
