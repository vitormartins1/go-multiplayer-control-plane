# Spec: 001-session-core

**Serviço:** session-service
**Status:** Done
**Data:** 2026-06-28

## Contexto

Núcleo do session-service (≈ GameLift — orquestração de frota/sessão; capstone do control plane, ADR-0010). A partir de um **match** (do matchmaking), aloca um **game server**, emite um **connect token Ed25519** por jogador (que o Unity Dedicated Server valida — data plane, ADR-0010/0011), e recebe o **resultado** da partida (callback do game server), disparando o fan-out para os serviços de dados.

É **control plane interno**: o cliente fala com o matchmaking, não com o session — por isso o MVP **não** usa o JWT de jogador (auth serviço-a-serviço é evolução). Protocolo **gRPC** (ADR-0003), unary no MVP; HTTP só para o JWKS do connect token, `/healthz` e `/metrics`.

Decisões de escopo (ver "Fora do escopo"): **allocator mock** em memória (Agones real é deploy); **fan-out via stub** que loga (o fan-out HTTP real exige endpoints server-side + service-auth, próxima feature); estado das sessões **em memória**.

## Comportamento

### Serviço gRPC
```
service Session {
  rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse);
  rpc ReportResult(ReportResultRequest) returns (ReportResultResponse);
}
```

### Criar sessão (a partir de um match)
- WHEN `CreateSession{matchId, players[]}` -> aloca um game server livre; emite **um connect token por jogador**; registra a sessão; retorna `{sessionId, serverAddress, grants[]}` (cada grant = `{playerId, connectToken}`).
- IF não houver game server livre -> `ResourceExhausted` (**ErrNoServer**).
- THE SYSTEM SHALL alocar cada game server a **no máximo uma** sessão por vez (alocação atômica — GO-LEARNINGS #01).

### Connect token (por jogador)
- THE SYSTEM SHALL emitir um JWT **EdDSA** com claims, no mínimo: `sub` (playerID), `match_id`, `server` (endereço) e `exp` (GO-LEARNINGS #02).
- THE SYSTEM SHALL expor a chave pública via `GET /.well-known/jwks.json` para o game server validar o token **offline** (sem chamar o backend).
- THE SYSTEM SHALL usar uma chave Ed25519 **própria do session** (distinta da do identity).

### Receber resultado e finalizar
- WHEN `ReportResult{matchId, results[]}` -> dispara o fan-out do resultado (Port `ResultPublisher`), **libera** o game server da sessão e encerra a sessão.
- IF não existir sessão para o `matchId` -> `NotFound` (**ErrSessionNotFound**).

## Contratos de Interface

### SessionService (Port de entrada)
- `Create(ctx, matchID string, players []string) (*Session, []ConnectGrant, error)`
- `ReportResult(ctx, matchID string, results []PlayerResult) error`

### ServerAllocator (Port de saída)
- `Allocate(ctx) (Server, error)` — `ErrNoServer` se o pool esgotar
- `Release(ctx, serverID string) error`

### ConnectTokenIssuer (Port de saída)
- `Issue(playerID, matchID, serverAddr string) (token string, error)` — JWT EdDSA
- `JWKS(ctx) ([]byte, error)` — chave pública

### ResultPublisher (Port de saída)
- `Publish(ctx, matchID string, results []PlayerResult) error` — fan-out (adapter loga no MVP)

### Tipos (domínio)
- `Session{ ID, MatchID string; Players []string; ServerID, ServerAddress string }`
- `Server{ ID, Address string }`
- `ConnectGrant{ PlayerID, Token string }`
- `PlayerResult{ PlayerID string; Score int64; Won bool }`

## Modelo de Estado

- **Sessões ativas:** `map[matchID]*Session` protegido por `sync.Mutex` (em memória).
- **Pool de game servers (mock):** slice de `Server` com flag de alocado, sob `sync.Mutex` — alocação/liberação atômicas.

## Casos de Teste Obrigatórios

- [ ] `CreateSession` com N players -> N grants, cada connect token verificável com a chave pública e com `sub`/`match_id`/`server`/`exp` corretos
- [ ] `CreateSession` retorna o `serverAddress` alocado; dois CreateSession não recebem o mesmo server enquanto não liberado
- [ ] Pool esgotado -> `ErrNoServer`
- [ ] `ReportResult` chama o `ResultPublisher`, libera o server (permite nova alocação) e encerra a sessão
- [ ] `ReportResult` de `matchId` inexistente -> `ErrSessionNotFound`
- [ ] Connect token assinado pela chave do session é validado pelo JWKS exposto

## Fora do escopo

- **Alocação real via Agones** (allocator é mock; Agones entra no deploy — ADR-0011/0012)
- **Fan-out HTTP real** para leaderboard/achievements (stub que loga no MVP; o real exige endpoints server-side + **service-to-service auth**, provável ADR — próxima feature)
- **Auth serviço-a-serviço** (CreateSession/ReportResult são control plane interno no MVP)
- **Integração matchmaking → session** (o matchmaking segue com connect-info mock)
- **Persistência** de sessões/resultados (memória no MVP)
- **Reconnection / spectator / migração de sessão** — backlog
