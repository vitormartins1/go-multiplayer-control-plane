# cloud-save-service

≈ **GameKit Game State Saving**. Guarda o **arquivo de save** do jogador — um blob opaco — em **object storage** (MinIO local / S3 em prod). Primeiro serviço sem Postgres (polyglot, ADR-0015).

- **Protocolo:** REST · **Porta host:** 8084
- **Storage:** MinIO / S3 (bucket `cloud-save`, um objeto por jogador) via `aws-sdk-go-v2`
- **Dependências:** identity (JWKS), MinIO

## API (autenticada por Bearer JWT)

| Método | Rota | Descrição |
|---|---|---|
| PUT | `/me/save` | grava o save (corpo = blob; `Content-Type` preservado; limite 5 MiB → 413) |
| GET | `/me/save` | carrega o save (404 se ausente) |

## Modelo de dados

**Object storage** (MinIO/S3) — não há tabelas; cada save é um objeto:

| Item | Valor |
|---|---|
| Bucket | `cloud-save` |
| Object key | `player_id` (`sub` do JWT) — **um objeto por jogador** |
| Body | blob do save (opaco) |
| Metadata | `Content-Type` (+ `LastModified` do storage) |

> Múltiplos slots (`player_id/{slot}`) e versionamento são evolução (FEATURE-MAP "próximo").

## Observabilidade

`/metrics` na porta 8084 (**Service Overview**, `$job=cloud-save-service`). Tracing no Tempo. Storage no dashboard **MinIO** (métricas Prometheus nativas do MinIO, sem exporter). Console do MinIO: http://localhost:9001 (`minioadmin`/`minioadmin`).

## Testar / Specs

- Manual: [`api/cloud-save.http`](api/cloud-save.http)
- Specs: [`specs/`](specs/) — `001-cloud-save-core`
