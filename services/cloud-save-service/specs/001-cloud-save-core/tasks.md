# Tasks: 001-cloud-save-core
**Plan:** specs/001-cloud-save-core/plan.md
**Data:** 2026-06-28

> Ordem: config → domain → service → storage(S3) → transport → wire-up/infra → collection.
> Testes acompanham cada camada. Marcar [x] conforme implementa.

## Fase 0 — Base compartilhada
- [x] `shared/config/config.go` — `S3Endpoint`, `S3AccessKey`, `S3SecretKey`, `S3Bucket`

## Fase 1 — Domain
- [x] `internal/domain/cloud_save.go` — `Snapshot{Blob, ContentType}` + Ports `CloudSaveService`, `SaveStore`, `TokenVerifier`
- [x] `internal/domain/errors.go` — `ErrNotFound`

## Fase 2 — Service
- [x] `internal/service/cloud_save_service.go` — `Save`/`Load` (delega ao `SaveStore`)
- [x] `internal/service/cloud_save_service_test.go` — unit (mock store): save→load; load inexistente→ErrNotFound; propagação de erro

## Fase 3 — Storage (adapter S3/MinIO)
- [x] `internal/storage/s3_store.go` — `aws-sdk-go-v2` (path-style, BaseEndpoint, credenciais estáticas); `Put`, `Get`, ensure bucket; `NoSuchKey`→`ErrNotFound`
- [x] `internal/storage/s3_store_test.go` — integration (testcontainers-minio): put/get round-trip (blob + content-type); sobrescrita; get inexistente→ErrNotFound; isolamento por jogador

## Fase 4 — Transport
- [x] `internal/transport/http_handler.go` — middleware de auth (`shared/auth`); `PUT /me/save` (`MaxBytesReader`→413; preserva Content-Type); `GET /me/save` (blob + content-type, 404); erros→HTTP
- [x] `internal/transport/http_handler_test.go` — httptest: PUT→GET (bytes + content-type); 404; corpo grande→413; sem token→401; isolamento

## Fase 5 — Wire-up, infra e verificação
- [x] `cmd/server/main.go` — wire-up (config, logger, tracing, S3 store, service, handler, shared/auth) + cadeia observável + graceful shutdown
- [x] `services/cloud-save-service/Dockerfile`
- [x] `infra/docker/docker-compose.yml` — `minio` (MINIO_PROMETHEUS_AUTH_TYPE=public) + `cloud-save-service` (env S3_*, JWKS, OTEL, depends_on)
- [x] `infra/docker/prometheus.yml` — jobs `cloud-save-service` e `minio` (`/minio/v2/metrics/cluster`)
- [x] `go test ./... -race -cover` + `-tags=integration` verdes; `go build ./...`
- [x] Validar no Docker: save/load com token do identity; 404; limite de tamanho; trace; métricas do MinIO no Prometheus

## Fase 6 — Collection
- [x] `services/cloud-save-service/api/cloud-save.http` — login no identity → token → PUT/GET /me/save
