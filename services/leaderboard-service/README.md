# leaderboard-service

≈ **ref. arch DynamoDB/Redis** (ElastiCache). Ranking global em tempo real com **Redis sorted sets**: submissão **highscore** (só sobe se for maior), top N e posição do jogador (rank ± k). Polyglot do ADR-0015.

- **Protocolo:** REST · **Porta host:** 8085
- **Storage:** Redis (sorted set `leaderboard:global`) via `go-redis/v9`
- **Dependências:** identity (JWKS), Redis

## API (autenticada por Bearer JWT)

| Método | Rota | Descrição |
|---|---|---|
| POST | `/scores` | submete um score (`{ "score": N }`); highscore; retorna `{rank, score}` |
| GET | `/leaderboard?limit=N` | top N (rank, playerId, score) |
| GET | `/leaderboard/me?around=k` | o jogador ± k posições (404 se sem score) |

## Modelo de dados

**Redis sorted set** `leaderboard:global` — não há tabelas:

| Item | Valor |
|---|---|
| member | `player_id` (`sub` do JWT) |
| score | pontuação (maior = melhor) |
| operações | `ZADD GT` (highscore), `ZREVRANGE WITHSCORES` (top/around), `ZREVRANK` (posição) |

> Dados efêmeros no MVP (sem AOF/RDB). Janelas temporais (chaves com TTL diário/semanal) são evolução.

## Observabilidade

`/metrics` na porta 8085 (**Service Overview**, `$job=leaderboard-service`). Tracing no Tempo. Storage no dashboard **Redis** (via `redis-exporter`).

## Testar / Specs

- Manual: [`api/leaderboard.http`](api/leaderboard.http)
- Specs: [`specs/`](specs/) — `001-leaderboard-core`
