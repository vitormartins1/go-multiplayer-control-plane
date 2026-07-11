# Spec: 001-leaderboard-core

**Serviço:** leaderboard-service
**Status:** Done
**Data:** 2026-06-28

## Contexto

Núcleo do leaderboard-service (≈ ref. arch DynamoDB/Redis): ranking global em tempo real por pontuação. Usa **Redis sorted sets** — o polyglot que o ADR-0015 previu para ranking real-time (par AWS = ElastiCache/DynamoDB). Valida o JWT do identity via `shared/auth`.

Decisões de escopo (ver "Fora do escopo"): **um leaderboard global**; **highscore** (mantém o melhor score do jogador); o score é submetido pelo próprio jogador (mock até o session existir — mesmo trust boundary do achievements).

## Comportamento

### Autenticação (todos os endpoints)
- THE SYSTEM SHALL exigir `Authorization: Bearer <jwt>` válido (JWKS do identity); senão -> 401 **ErrUnauthorized**.
- THE SYSTEM SHALL identificar o jogador pelo `sub` do token, nunca por um id do path/corpo.

### Submeter score
- WHEN POST /scores com `{ "score": <inteiro> }` -> registra o score do jogador **apenas se for maior** que o atual (highscore) e retorna 200 com `{ rank, score }` (a posição e o melhor score do jogador após a submissão).
- IF o corpo não tiver um `score` inteiro válido -> 400 **ErrInvalidScore**.
- THE SYSTEM SHALL nunca rebaixar o jogador ao receber um score menor que o já registrado.

### Ranking global (top N)
- WHEN GET /leaderboard?limit=N -> 200 com as N maiores entradas, ordenadas por score decrescente: `[{ rank, playerId, score }]` (rank 1-based).
- THE SYSTEM SHALL aplicar um limite default (10) e um teto (100) ao `N`.

### Ranking ao redor do jogador
- WHEN GET /leaderboard/me?around=k -> 200 com o jogador e até `k` posições acima e `k` abaixo: `[{ rank, playerId, score }]`.
- IF o jogador ainda não tem score -> 404 **ErrNotRanked**.

## Contratos de Interface

### LeaderboardService (Port de entrada)
- `Submit(ctx, playerID string, score int64) (Entry, error)` — aplica highscore; retorna a entry do jogador
- `Top(ctx, limit int) ([]Entry, error)`
- `Around(ctx, playerID string, k int) ([]Entry, error)`

### LeaderboardStore (Port de saída)
- `Submit(ctx, playerID string, score int64) error` — `ZADD GT`
- `Rank(ctx, playerID string) (rank int64, score int64, err error)` — `ErrNotRanked` se ausente
- `Top(ctx, limit int) ([]Entry, error)`
- `Range(ctx, start, stop int64) ([]Entry, error)` — janela por posição (para o "around")

### TokenVerifier (Port de saída, usada no transport)
- `Verify(ctx, token) (playerID string, error)` — `shared/auth` (JWKS do identity).

### Tipo
- `Entry{ Rank int64; PlayerID string; Score int64 }` — rank 1-based.

## Modelo de Armazenamento

**Redis sorted set (ADR-0015)**

| Item | Valor |
|---|---|
| Key | `leaderboard:global` |
| Member | `player_id` (`sub` do JWT; aceita `guest-…`) |
| Score | a pontuação (maior = melhor) |
| Operações | `ZADD GT` (submeter), `ZREVRANGE WITHSCORES` (top/around), `ZREVRANK`/`ZSCORE` (rank do jogador) |

## Regras de Validação

| Item | Regra | Erro |
|---|---|---|
| Token | válido (JWKS), não expirado | ErrUnauthorized |
| score | inteiro presente no corpo | ErrInvalidScore (400) |
| limit | clamp para [1, 100], default 10 | — |
| around (k) | clamp para [0, 50], default 5 | — |

## Casos de Teste Obrigatórios

- [ ] POST /scores grava; GET /leaderboard mostra o jogador com o score e rank corretos
- [ ] Highscore: submeter um score menor não rebaixa (mantém o maior); submeter maior atualiza
- [ ] Top N vem ordenado por score decrescente, rank 1-based
- [ ] Around retorna o jogador no centro com até k acima/abaixo
- [ ] GET /leaderboard/me sem score do jogador -> 404 ErrNotRanked
- [ ] corpo sem score inteiro -> 400 ErrInvalidScore
- [ ] requisição sem Bearer / token inválido -> 401

## Fora do escopo

- **Janelas temporais** (diário/semanal/all-time com TTL) — próxima feature
- **Múltiplos leaderboards** (por modo/temporada) — próxima feature
- **Validação / anti-cheat de score** — backlog
- **Empates / tie-breaking** determinístico além da ordem natural do Redis
