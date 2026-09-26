# leaderboard-service

≈ **ref. arch DynamoDB/Redis** (ElastiCache). Ranking em tempo real com **Redis sorted sets**, multi-tenant por jogo (ADR-0017): submissão **highscore** (só sobe se for maior), top N e posição do jogador (rank ± k). Cada jogo tem N **boards** (métricas) × 4 **janelas temporais** (spec 007). Alimentado tanto por chamada direta do jogo (Bearer JWT) quanto pelo **fan-out durável** via SQS (ADR-0020) — um consumer separado (`cmd/consumer`) para os resultados de partida. Polyglot do ADR-0015.

- **Protocolo:** REST · **Porta host:** 8085 (API) — consumer roda como processo separado (`/healthz`+`/metrics` próprios, sem porta HTTP de negócio)
- **Storage:** Redis (um sorted set por `game_id` × `board` × `window`) via `go-redis/v9`
- **Fan-out:** consumer SQS (fila `match-results-leaderboard`, ADR-0020) despacha por `kind` do envelope — ver "Arquétipos suportados"
- **Dependências:** identity (JWKS, rotas de jogador), Redis, SQS/LocalStack (consumer)
- **Multi-tenant por jogo** (ADR-0017): toda rota é escopada por `game_id` — header `X-Game-Id` (jogador) ou `{gameId}` no path (admin).

## API

Endpoints de jogador: Bearer JWT + header `X-Game-Id` (ausente → `400 missing_game`).
Endpoints admin: internal key `X-Internal-Key` (ADR-0018), sem JWT — jogo/jogador no path.

Query params comuns: `?board=<nome>` (default `score`; regex `^[a-z][a-z0-9_]*$`, mesmo identificador do `scoringStats` no game-registry) e `?window=alltime|daily|weekly|monthly` (default `alltime`).

| Método | Rota | Auth | Descrição |
|---|---|---|---|
| POST | `/scores` | JWT + X-Game-Id | submete um score (`{ "score": N }`) no board pedido; highscore (`ZADD GT`); retorna `{rank, playerId, score}` (rank do all-time) |
| GET | `/leaderboard?board=&window=&limit=&offset=` | JWT + X-Game-Id | página do top N do board/janela pedidos |
| GET | `/leaderboard/me?board=&window=&around=k` | JWT + X-Game-Id | o jogador ± k posições no board/janela pedidos (`404 not_ranked` se sem score) |
| POST | `/admin/games/{gameId}/players/{playerID}/scores?board=` | X-Internal-Key | aplica highscore server-authoritative (é o que o consumer do fan-out usa) |
| PUT | `/admin/games/{gameId}/players/{playerID}/scores?board=` | X-Internal-Key | substitui o score independente do valor atual (override; `ZADD` sem `GT`) |
| GET | `/admin/games/{gameId}/leaderboard?board=&window=&limit=&offset=` | X-Internal-Key | top N do jogo (consumo do painel admin) |

## Modelo de dados

**Redis sorted set** por `(game_id, board, window)` — não há tabelas:

| Item | Valor |
|---|---|
| Key | `leaderboard:{gameId}:{board}` (all-time, key pré-janelas intacta) + sufixo de período nas janelas: `:d:YYYYMMDD` (daily), `:w:YYYYWww` (weekly, semana ISO), `:m:YYYYMM` (monthly) |
| member | `player_id` (`sub` do JWT, ou o `playerID` do path/envelope nas rotas admin/fan-out) |
| score | pontuação (maior = melhor) |
| operações | `ZADD GT` (highscore, `Submit`), `ZADD` sem GT (`ForceScore`, admin override), `ZREVRANGE WITHSCORES` (top/around), `ZREVRANK` (posição), `ZINCRBY` (boards cumulativos: `wins`, `top_n_finishes`, `missions_completed`, `<stat>_total`) |
| idempotência do cumulativo | uma key `leaderboard:processed:{gameId}:{matchId}` com `SET NX EX` de 30 dias (spec 008), atômica com os `ZINCRBY` via script Lua — reprocessar a mesma partida (SQS at-least-once) é no-op; o SET legado `leaderboard:processed:{gameId}` só é consultado |
| TTL das janelas | daily 3d, weekly 15d, monthly 45d (sliding, renovado a cada write); all-time sem TTL |

Todo **write** de highscore (`Submit`/`ForceScore`) espalha nas 4 janelas simultaneamente (pipeline Redis); toda **leitura** aponta para a janela pedida (`?window=`, default `alltime`). Compat total com o contrato pré-spec-007 quando `board`/`window` são omitidos.

## Arquétipos suportados

O consumer do fan-out (`cmd/consumer`) despacha o envelope da fila `match-results-leaderboard` pelo campo `kind` (ADR-0023):

| `kind` | Arquétipo | O que alimenta |
|---|---|---|
| `win_loss_draw` (ou envelope v1 legado, sem `kind`, com `matchId`) | `queue_arena` | highscore `score` + um board por stat; cumulativo `wins` (outcome `win`) e `<stat>_total` |
| `placement` | `battle_royale` | highscore `survived_ms` + um board por stat; cumulativo `wins` (1º lugar), `top_n_finishes` (colocação ≤ 10) e `<stat>_total` |
| `collective` | `coop_mission` | highscore por stat + cumulativo `<stat>_total`; `missions_completed` (+1 para todo o time em `MISSION_SUCCESS` — desfecho coletivo da sessão) |

**`host_world_dropin` não é rankeado hoje.** O arquétipo publica eventos por participante (`participation_event` — `JOINED`/`BOSS_DEFEATED`/`HOST_KILLED`/`DIED`/`LEFT`, sem placar terminal); o `kind` `participation_event` cai no `default` do dispatch — **é logado em `Debug` ("envelope ignorado: kind não rankeável") e a mensagem é deletada da fila sem erro**, ou seja, descartado silenciosamente e de propósito, não um bug. Qualquer `kind` desconhecido tem o mesmo destino. Rankear mundos persistentes por evento (em vez de resultado terminal de partida) é decisão de design ainda não tomada — ver [`docs/FEATURE-MAP.md`](../../docs/FEATURE-MAP.md) ("ranking de invasões/co-ops (host_world_dropin, cauda)").

## Observabilidade

`/metrics` na porta 8085 (API) e na porta HTTP própria do consumer (**Service Overview**, `$job=leaderboard-service` / `$job=leaderboard-consumer`). Consumer expõe também `/healthz`. Métricas RED do fan-out: `fanout_consume_total{result=ok|dup|error}` e `fanout_consume_duration_seconds` (ADR-0020). Tracing no Tempo. Storage no dashboard **Redis** (via `redis-exporter`).

## Testar / Specs

- Manual: [`api/leaderboard.http`](api/leaderboard.http)
- Specs: [`specs/`](specs/) — `001-leaderboard-core`, `002-game-scope`, `003-metric-boards`, `004-cumulative-boards`, `005-battle-royale-placement`, `006-coop-mission-collective`, `007-time-windows`
