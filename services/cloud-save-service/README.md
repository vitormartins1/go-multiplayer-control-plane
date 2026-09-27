# cloud-save-service

≈ **GameKit Game State Saving**. Guarda o **arquivo de save** do jogador — um blob opaco — em **object storage** (S3 do LocalStack no local / S3 em prod). Primeiro serviço sem Postgres (polyglot, ADR-0015).

- **Protocolo:** REST · **Porta host:** 8084
- **Storage:** S3 (LocalStack no local) (bucket `cloud-save`, um objeto por jogo+jogador) via `aws-sdk-go-v2`
- **Dependências:** identity (JWKS), S3
- **Multi-tenant por jogo** (ADR-0017): escopo por `game_id` (prefixo da key).

## API

Endpoints de jogador: Bearer JWT + header `X-Game-Id` (ausente → `400 missing_game`).
Endpoint admin: internal key `X-Internal-Key` (ADR-0018), sem JWT.

| Método | Rota | Auth | Descrição |
|---|---|---|---|
| PUT | `/me/save` | JWT + X-Game-Id | grava o save (corpo = blob; `Content-Type` preservado; limite 5 MiB → 413) |
| GET | `/me/save` | JWT + X-Game-Id | carrega o save (404 se ausente) |
| GET | `/admin/games/{gameId}/players/{playerID}/save` | internal key | save do jogador no jogo: metadados + blob base64 (404 se ausente) |

## Modelo de dados

**Object storage** (S3) — não há tabelas; cada save é um objeto:

| Item | Valor |
|---|---|
| Bucket | `cloud-save` |
| Object key | `{game_id}/{player_id}` — **um objeto por jogo+jogador** |
| Body | blob do save (opaco) |
| Metadata | `Content-Type` (+ `LastModified` do storage → `updatedAt` no admin read) |

> Múltiplos slots (`{game_id}/{player_id}/{slot}`) e versionamento são evolução (FEATURE-MAP "próximo").

## Observabilidade

`/metrics` na porta 8084 (**Service Overview**, `$job=cloud-save-service`). Tracing no Tempo. O S3 local é o LocalStack (`localstack:4566`, credenciais `test`/`test`); até 2026-09 era um MinIO, cuja imagem saiu do Docker Hub. Os saves locais não persistem entre restarts do LocalStack.

## Testar / Specs

- Manual: [`api/cloud-save.http`](api/cloud-save.http)
- Specs: [`specs/`](specs/) — `001-cloud-save-core`, `002-game-scope`
