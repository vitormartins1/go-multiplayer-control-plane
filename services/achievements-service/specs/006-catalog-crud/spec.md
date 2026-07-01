# Spec: 006-catalog-crud

**Serviço:** achievements-service + admin-bff + admin-ui
**Status:** Implementada (2026-06-30)
**Data:** 2026-06-30

## Contexto

Completa o CRUD do catálogo de conquistas no painel: hoje só **cria** (004/admin). Adiciona **editar** (corrigir name/description/eventType/target) e **remover** uma conquista de um jogo. Self-contained por jogo (sem cascata cross-service — diferente de apagar jogo).

## Comportamento

### achievements-service (admin, internal key na borda)
- WHEN `PUT /admin/games/{gameId}/achievements/{id}` com `{name, description, eventType, target}` → atualiza a conquista. `id` (slug) e `gameId` são a chave, **não** mudam. `400` se inválido; `404` se não existir.
- WHEN `DELETE /admin/games/{gameId}/achievements/{id}` → remove a conquista **e o progresso dos jogadores nela** (transação, por causa da FK `player_achievements → achievements`). `404` se não existir. Idempotência não exigida (404 no 2º delete é ok).

### admin-bff
- `PUT /api/games/{g}/achievements/{id}` e `DELETE /api/games/{g}/achievements/{id}` → repassam; 400/404 repassados (UpstreamError); indisponibilidade → 502.

### admin-ui (aba Achievements do GameDetail)
- Cada conquista do catálogo ganha **Editar** (modal pré-preenchido; salva via PUT) e **Remover** (confirmação; DELETE). Sucesso recarrega o catálogo; erros (400/404/502) viram mensagem.

## Contratos
- domain: `AchievementService.UpdateAchievement/DeleteAchievement`; `AchievementRepository.UpdateAchievement/DeleteAchievement`; novo `ErrAchievementNotFound` (→ 404).
- repo: Update = `UPDATE ... WHERE game_id,id` (0 linhas → NotFound); Delete = tx (apaga `player_achievements` da conquista, depois `achievements`).
- admin-bff: `AchievementsReader.UpdateAchievement/DeleteAchievement`; adapter PUT/DELETE; rotas.
- admin-ui: `api.updateAchievement/deleteAchievement`; UI no catálogo.

## Casos de Teste Obrigatórios
- [ ] (service) update valida (name/eventType/target) e delega; delete delega
- [ ] (repo, integração) update altera campos; update inexistente → NotFound; delete remove conquista + progresso; delete inexistente → NotFound
- [ ] (transport) PUT 200 / 404 / 400; DELETE 204 / 404
- [ ] (admin-bff) PUT/DELETE 200/204 + 404 repassado + 502
- [ ] (ui) editar e remover no catálogo; build ok

## Fora do escopo
- Editar/remover **jogo** (apagar jogo tem dado órfão cross-service — decisão à parte)
- Mudar o `id`/slug de uma conquista (= criar outra)
