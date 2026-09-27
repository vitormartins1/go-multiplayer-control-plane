# Tasks: 007-sessions-panel

**Plan:** [plan.md](plan.md)
**Data:** 2026-07-03

- [x] `internal/session/client.go` + testes (httptest fake do session-service)
- [x] Handler `GET /api/games/{gameId}/sessions` + enriquecimento de handles (best-effort) + testes
- [x] `SESSION_URL` no main + compose + aresta na topologia
- [x] Commits `feat(admin-bff): sessions panel`, merge --no-ff

> **Revisão 2026-09:** checkboxes marcados retroativamente — o código existe na `main` (verificado na revisão de produto). Validação visual ao vivo, quando houver, segue pendente.
