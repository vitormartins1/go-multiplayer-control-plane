# Tasks 011 — Importação em lote de conquistas (admin-bff)

## Implementação

- [x] **1. Tipos no domain** — adicionar `ImportResult`, `ImportResponse` e constantes de status em `internal/domain/admin.go`
- [x] **2. Port** — adicionar `ImportAchievements(ctx, gameID, items) (ImportResponse, error)` à interface `AchievementsReader` em `internal/domain/admin.go`
- [x] **3. Client** — implementar `ImportAchievements` em `internal/achievements/client.go` (loop sequencial, classifica erros do upstream)
- [x] **4. Handler** — adicionar `importAchievements` handler em `internal/transport/http_handler.go` com validações de `empty_batch` e `batch_too_large`
- [x] **5. Rota** — registrar `POST /api/games/{gameId}/achievements/import` em `Register`, antes da rota `/{id}`
- [x] **6. Service** — adicionar `ImportAchievements` delegando ao reader em `internal/service/admin_service.go`
- [x] **7. Interface AdminService** — adicionar `ImportAchievements` ao contrato do transport

## Testes

- [x] **8. Testes do client** — `TestImportAchievements_TodosCriados`, `_DuplicataParcial`, `_ServicoFora_StatusError`, `_OrdemPreservada` em `internal/achievements/`
- [x] **9. Testes do handler** — `TestImportAchievements_BatchVazio_400`, `_BatchGrande_400`, `_BodyInvalido_400`, `_Sucesso_200`, `_Parcial_200` em `internal/transport/`
