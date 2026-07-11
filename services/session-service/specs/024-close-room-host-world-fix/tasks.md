# Tasks: 024-close-room-host-world-fix
**Plan:** specs/024-close-room-host-world-fix/plan.md
**Data:** 2026-07-11

## Fase 1 — session-service
- [ ] `resolveAdminCloseID(id)`: tenta `matchIndex[id]`, cai pra `sessions[id]`
- [ ] Atualizar `CloseByAdmin` pra usar a resolução nova
- [ ] Teste: force-close de sessão host_world_dropin (aberta via `OpenWorld`) por `sessionId` — sucesso (bug corrigido)
- [ ] Teste: force-close de sessão queue_arena por `matchId` — continua funcionando (regressão)
- [ ] Teste: force-close de id desconhecido (nem `matchId` nem `sessionId`) — 404/`ErrSessionNotFound`

## Fase 2 — admin-ui
- [ ] `Rooms.tsx`: `confirmClose` usa `closing.matchId || closing.sessionId`
- [ ] `tsc --noEmit` + `npm run build` limpos

## Fase 3 — Verificação
- [ ] `go test ./services/session-service/... -race`
- [ ] Smoke ao vivo: abrir mundo host_world_dropin de teste, encerrar pelo console admin (`:8089`), confirmar que a sala fecha e o slot do allocator libera
- [ ] Confirmação do usuário
