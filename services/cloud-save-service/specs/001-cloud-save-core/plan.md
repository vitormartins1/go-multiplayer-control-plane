# Plan: 001-cloud-save-core
**Spec:** specs/001-cloud-save-core/spec.md
**Data:** 2026-06-28

## Abordagem técnica

API REST com `net/http`, hexagonal (ADR-0002). `domain` define `Snapshot` e as Ports; `service` orquestra; `storage` é o adapter de saída para **object storage** (S3/MinIO); `transport` traduz HTTP, valida o tamanho e preserva o `Content-Type`. O blob é opaco — o serviço não o interpreta. Auth via `shared/auth` (mesma Port estrutural dos demais).

## Storage e compute (ADR-0005, ADR-0015)

**Primeiro desvio do Postgres** (justificado pelo ADR-0015: "blob grande → object store"): o save é um arquivo opaco, padrão de acesso put/get por chave — **object storage** é o ajuste natural, e o par AWS é o **S3**. Local: **MinIO** (S3-compatible). Em prod, o mesmo código aponta para o S3 real (só muda o endpoint).

## Arquivos a criar ou modificar

- `shared/config/config.go` — `S3Endpoint`, `S3AccessKey`, `S3SecretKey`, `S3Bucket`
- `services/cloud-save-service/internal/domain/cloud_save.go` — `Snapshot` + Ports `CloudSaveService`, `SaveStore`, `TokenVerifier`
- `.../internal/domain/errors.go` — `ErrNotFound`
- `.../internal/service/cloud_save_service.go` (+ test) — `Save`/`Load` (delega ao store)
- `.../internal/storage/s3_store.go` (+ integration test) — adapter `aws-sdk-go-v2` (Put/Get, ensure bucket)
- `.../internal/transport/http_handler.go` (+ httptest) — `PUT/GET /me/save`, auth, `MaxBytesReader`, content-type
- `.../cmd/server/main.go`, `.../Dockerfile`, `.../api/cloud-save.http`
- `infra/docker/docker-compose.yml` — `minio` + `cloud-save-service`
- `infra/docker/prometheus.yml` — jobs `cloud-save-service` e `minio`

## Dependências e libs

- `github.com/aws/aws-sdk-go-v2` (`config`, `credentials`, `service/s3`, `service/s3/types`) — cliente S3; o mesmo código serve S3 real e MinIO
- `github.com/testcontainers/testcontainers-go/modules/minio` — MinIO real e descartável na integração
- Reaproveita `shared/auth`, `shared/middleware`, `shared/telemetry`, otelhttp

## Decisões técnicas e trade-offs

- **`aws-sdk-go-v2/s3` (não minio-go):** fiel ao par S3 da tese — o mesmo código roda contra MinIO (local) e S3 (prod), só trocando o endpoint. Requer `UsePathStyle = true` e `BaseEndpoint` apontando ao MinIO; credenciais estáticas via `credentials.NewStaticCredentialsProvider`.
- **`NoSuchKey` → `domain.ErrNotFound`:** no `Get`, `errors.As(err, &*types.NoSuchKey)` mapeia "objeto ausente" para 404.
- **Ensure bucket no boot:** `HeadBucket`; se ausente, `CreateBucket` (idempotente) — evita setup manual no MinIO.
- **Limite de tamanho no transport:** `http.MaxBytesReader` (ex. 5 MiB); estouro vira `*http.MaxBytesError` → 413. Quota por jogador é backlog.
- **Content-Type preservado** como metadata do objeto S3 (`PutObject.ContentType`), devolvido no `GetObject`.
- **Object key = `player_id`** — um objeto por jogador (MVP). Múltiplos slots viram `player_id/{slot}`.
- **Observabilidade adaptada:** sem Postgres, não há `postgres-exporter`; em vez disso, o **MinIO expõe métricas Prometheus nativas** (`/minio/v2/metrics/cluster`, com `MINIO_PROMETHEUS_AUTH_TYPE=public`) — o storage entra na stack pelo seu próprio endpoint. O serviço continua no Service Overview (RED) e no tracing como os demais.

## Ordem de implementação

1. `shared/config` (envs S3)
2. `domain` (Snapshot, Ports, errors)
3. `service` (Save/Load) + unit tests (mock store)
4. `storage` (adapter S3) + integration tests (testcontainers-minio)
5. `transport` (MaxBytesReader, content-type, auth) + httptest
6. wire-up `main` + Dockerfile + compose (minio + service) + jobs no Prometheus
7. collection `.http`
8. `go test ./... -race` + integração verdes; `go build ./...`; validar no Docker (save/load com token, 404, limite)

## Riscos e pontos de atenção

- **Config do aws-sdk-go-v2 para MinIO:** path-style + BaseEndpoint + região fictícia (`us-east-1`) — pegadinhas comuns; validar no teste de integração.
- **Detecção de `NoSuchKey`:** o tipo de erro pode variar; confirmar contra o MinIO real (testcontainers) e ajustar (`*types.NoSuchKey` vs `smithy.APIError` code).
- **testcontainers exige Docker.**
- **Tamanho do aws-sdk-go-v2** no módulo (várias dependências novas) — aceitável, é a fundação de object storage para os próximos serviços.
