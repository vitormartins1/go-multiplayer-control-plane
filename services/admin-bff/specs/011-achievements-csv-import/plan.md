# Plan 011 — Importação em lote de conquistas (admin-bff)

## Decisões técnicas

### Processamento sequencial no BFF

O BFF itera sobre o array e chama `achievements-service` uma vez por item.  
Para o volume esperado (≤ 500 linhas) e uso admin, a latência acumulada (~5-50ms × N) é aceitável.  
Paralelismo aumentaria complexidade sem ganho significativo no cenário de uso.

### Sem novo método no achievements-service

O achievements-service já expõe `POST /admin/games/{gameId}/achievements` (criação unitária).  
O BFF reutiliza o `AchievementsClient.CreateAchievement` existente, classificando os erros retornados.

### Sem mudança no domain do BFF

`domain.Achievement` já tem todos os campos necessários para criar uma conquista.  
Adicionamos apenas os tipos `ImportResult` e `ImportResponse` no domain.

## Arquivos a modificar

```
services/admin-bff/
  internal/domain/admin.go          ← adicionar ImportResult, ImportResponse, constantes de status
  internal/achievements/client.go   ← adicionar ImportAchievements (loop + classifica erros)
  internal/transport/http_handler.go ← adicionar importAchievements handler + registrar rota
```

Nenhum arquivo novo necessário.

## Novos tipos (domain/admin.go)

```go
const (
    ImportStatusCreated   = "created"
    ImportStatusDuplicate = "duplicate"
    ImportStatusInvalid   = "invalid"
    ImportStatusError     = "error"
)

type ImportResult struct {
    Row    int    `json:"row"`
    ID     string `json:"id"`
    Status string `json:"status"`
    Error  string `json:"error,omitempty"`
}

type ImportResponse struct {
    Created int            `json:"created"`
    Failed  int            `json:"failed"`
    Results []ImportResult `json:"results"`
}
```

## AchievementsReader (domain/admin.go — interface)

Adicionar ao port `AchievementsReader`:

```go
ImportAchievements(ctx context.Context, gameID string, items []Achievement) (ImportResponse, error)
```

## Implementação no client (achievements/client.go)

```go
func (c *Client) ImportAchievements(ctx context.Context, gameID string, items []Achievement) (domain.ImportResponse, error) {
    resp := domain.ImportResponse{Results: make([]domain.ImportResult, 0, len(items))}
    for i, a := range items {
        row := i + 1
        _, err := c.CreateAchievement(ctx, gameID, a)
        result := domain.ImportResult{Row: row, ID: a.ID}
        switch {
        case err == nil:
            result.Status = domain.ImportStatusCreated
            resp.Created++
        case isUpstreamConflict(err):
            result.Status = domain.ImportStatusDuplicate
            result.Error = "achievement_exists"
            resp.Failed++
        case isUpstreamBadRequest(err):
            result.Status = domain.ImportStatusInvalid
            result.Error = upstreamErrorCode(err)
            resp.Failed++
        default:
            result.Status = domain.ImportStatusError
            result.Error = "service_unavailable"
            resp.Failed++
        }
        resp.Results = append(resp.Results, result)
    }
    return resp, nil
}
```

O `ImportAchievements` nunca retorna `error` não-nil ao chamador — falhas de linha são absorvidas no `ImportResponse`. Só retornaria erro se houvesse problema antes de iterar (ex.: body inválido), mas esse caso é tratado antes da chamada.

## Handler HTTP (transport/http_handler.go)

```go
func (h *Handler) importAchievements(w http.ResponseWriter, r *http.Request) {
    gameID := r.PathValue("gameId")

    var body struct {
        Achievements []domain.Achievement `json:"achievements"`
    }
    if err := decode(r, &body); err != nil {
        writeError(w, http.StatusBadRequest, "invalid_input")
        return
    }
    if len(body.Achievements) == 0 {
        writeError(w, http.StatusBadRequest, "empty_batch")
        return
    }
    if len(body.Achievements) > 500 {
        writeError(w, http.StatusBadRequest, "batch_too_large")
        return
    }

    result, err := h.achievements.ImportAchievements(r.Context(), gameID, body.Achievements)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "service_unavailable")
        return
    }
    writeJSON(w, http.StatusOK, result)
}
```

Rota registrada em `Register`:

```go
mux.HandleFunc("POST /api/games/{gameId}/achievements/import", h.importAchievements)
```

A rota específica `/import` deve ser registrada **antes** de `/api/games/{gameId}/achievements/{id}` para evitar conflito de matching (o ServeMux do Go 1.22 usa longest-match, mas `/import` pode ser confundido com um `id`).

## Testes

`internal/achievements/client_test.go` (ou arquivo dedicado):
- `TestImportAchievements_AllCreated` — todos 201
- `TestImportAchievements_PartialDuplicate` — mix 201/409
- `TestImportAchievements_InvalidRow` — 400 do achievements-service
- `TestImportAchievements_EmptyBatch` — rejeitado no handler (400)
- `TestImportAchievements_TooLarge` — 501 itens → 400

`internal/transport/http_handler_test.go`:
- `TestImportAchievements_EmptyBatch`
- `TestImportAchievements_TooLarge`
- `TestImportAchievements_Success`
