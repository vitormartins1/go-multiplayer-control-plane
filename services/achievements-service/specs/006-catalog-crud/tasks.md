# Tasks: 006-catalog-crud
**Spec:** specs/006-catalog-crud/spec.md
**Data:** 2026-06-30

## achievements-service
- [x] domain: Service/Repository Update+Delete; ErrAchievementNotFound
- [x] repo: UpdateAchievement (0 linhas→NotFound); DeleteAchievement (tx: progresso+catálogo) + integração
- [x] service: UpdateAchievement (valida) + DeleteAchievement + unit
- [x] transport: PUT/DELETE /admin/games/{g}/achievements/{id}; 404 no writeDomainError; catálogo expõe eventType + testes

## admin-bff
- [x] AchievementsReader Update/Delete; adapter PUT/DELETE; service; rotas PUT/DELETE + testes; catalogResponse decodifica eventType

## admin-ui
- [x] api.updateAchievement/deleteAchievement (+ CatalogAchievement.eventType)
- [x] GameAchievements: editar (modal pré-preenchido) + remover (confirmação) por conquista

## verificação
- [x] go test ./... -race + build/vet; npm run build
- [x] Docker E2E: create/edit/delete via BFF; 404 em inexistente; eventType no catálogo

## Follow-up 1 — UpdateAchievement reseta progresso quando o critério muda

Achado no `/code-review` do projeto como um todo (2026-07-10): `UpdateAchievement`
só escrevia a linha do catálogo (`internal/repository/postgres_achievement.go`);
`player_achievements` nunca era tocado. O critério (`event_type`/`stat_key`/
`stat_mode`) define o que o `progress` acumulado está medindo — trocar o
critério (ex.: de stat `kills >= 10` pra evento `match_won x3`, spec
008-stat-criteria) deixava o progresso antigo órfão: um jogador com
`progress=10` (kills) reaparecia "quase lá" (ou já desbloqueável) num critério
de vitórias que nunca cumpriu, sem qualquer revalidação.

- `internal/repository/postgres_achievement.go`: `UpdateAchievement` virou uma
  transação — `SELECT ... FOR UPDATE` lê o critério antigo (evita corrida com
  update concorrente), aplica o `UPDATE` do catálogo e, se
  `event_type`/`stat_key`/`stat_mode` mudou, `DELETE FROM player_achievements`
  pra aquela conquista (mesmo padrão de `DeleteAchievement`, que já fazia isso
  na remoção). Trocar só `Target`/`Name`/`Description` (mesmo critério) não
  reseta nada.
- Testes novos (integração): `TestUpdateAchievement_TrocaCriterio_ResetaProgresso`
  e `TestUpdateAchievement_MesmoCriterio_PreservaProgresso` provam os dois
  lados do comportamento.

Barra verde: suíte completa (949 testes) + integração do achievements-service
(35 testes, incl. os 2 novos).
