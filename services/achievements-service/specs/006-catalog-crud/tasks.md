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
