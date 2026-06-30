# Plan: 002-game-scope
**Spec:** specs/002-game-scope/spec.md
**Data:** 2026-06-30

## Abordagem técnica
`game_id` entra nos métodos (domain/service/storage/transport). A key S3 vira
`{gameID}/{playerID}` (prefixo por jogo). Player-facing pega do `X-Game-Id`;
admin do path + internal key. Sem migração de objetos antigos (S3/MinIO dev).

## Arquivos
- `internal/domain/cloud_save.go` — `gameID` em Service/Store; novo `LoadFor(ctx, gameID, playerID)` (admin)
- `internal/storage/s3_store.go` — `objectKey(gameID, playerID)`; `Put`/`Get` usam a key composta
- `internal/storage/s3_store_test.go` — isolamento por jogo (testcontainers MinIO, se houver; senão adapter fake)
- `internal/service/cloud_save_service.go` — repassa gameID; `LoadFor`
- `internal/transport/http_handler.go` — player: `gameFromHeader` (X-Game-Id, vazio->400); admin: `GET /admin/games/{gameId}/players/{playerID}/save` (404 se ausente)
- `cmd/server/main.go` — `auth.RequireInternalKey(key, "/admin")`; lê `INTERNAL_API_KEY`
- `infra/docker/docker-compose.yml` — `INTERNAL_API_KEY`
- `api/*.http` — `X-Game-Id` nos player + exemplo admin (X-Internal-Key)

## Decisões
- **Key por jogo** (prefixo) vs bucket por jogo: prefixo é mais simples e barato (ADR-0008); um bucket só.
- **Admin read** devolve snapshot (conteúdo + metadados) reusando `Get`.
- **Sem migração** dos objetos globais antigos (dev recomeça).

## Ordem
1. domain -> storage (key por jogo) + teste -> service -> transport
2. main (guard) + compose (env)
3. `go test -race` + build; Docker: isolamento por jogo + admin com/sem key

## Riscos
- **Teste de storage**: ver como o 001 testa o S3 (MinIO testcontainers vs fake) e seguir o mesmo.
- **Conteúdo binário no admin read**: definir Content-Type/encoding (JSON com base64 ou stream) no plan da implementação.
