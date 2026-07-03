# Tasks: 005-admin-sessions

**Plan:** [plan.md](plan.md)
**Data:** 2026-07-03

- [ ] Ring buffer de fechadas no `closeSession` (`SESSION_HISTORY_LIMIT`, mutex compartilhado) + testes
- [ ] `SessionsByGame(gameID)` (active + closed, desc)
- [ ] `GET /admin/games/{gameId}/sessions` (+ `state=`) no mux HTTP; DTO camelCase; httptest
- [ ] Env no main; README do serviço atualizado
- [ ] Commits `feat(session): admin sessions`, merge --no-ff
