# Tasks: 005-admin-sessions

**Plan:** [plan.md](plan.md)
**Data:** 2026-07-03

- [x] Ring buffer de fechadas no `closeSession` (`SESSION_HISTORY_LIMIT`, mutex compartilhado) + testes
- [x] `SessionsByGame(gameID)` (active + closed, desc)
- [x] `GET /admin/games/{gameId}/sessions` (+ `state=`) no mux HTTP; DTO camelCase; httptest
- [x] Env no main; README do serviço atualizado
- [x] Commits `feat(session): admin sessions`, merge --no-ff

> **Revisão 2026-09:** checkboxes marcados retroativamente — o código existe na `main` (verificado na revisão de produto). Validação visual ao vivo, quando houver, segue pendente.
