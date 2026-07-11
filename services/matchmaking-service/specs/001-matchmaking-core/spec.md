# Spec: 001-matchmaking-core

**Serviço:** matchmaking-service
**Status:** Done (2026-06-28)
**Data:** 2026-06-28

## Contexto

Núcleo do matchmaking-service (≈ GameLift FlexMatch): o jogador entra numa fila e acompanha o status do seu ticket **em tempo real** até a partida ser formada. É o primeiro serviço **gRPC** (server-streaming) do backend — os de CRUD são REST (ADR-0003). Valida o JWT do identity via `shared/auth` (interceptor gRPC).

Decisões de escopo (ver "Fora do escopo"): formação **FIFO por tamanho de time** (skill-based é a próxima feature); **fila em memória** (estado efêmero, `sync.Mutex` local — prod usaria Redis); ao formar o match, gera um **connect-info mock** (a chamada real ao session-service entra quando ele existir).

## Comportamento

### Serviço gRPC
`Matchmaking.FindMatch` — **server-streaming**: o cliente faz a chamada e recebe um fluxo de atualizações do ticket até o match (ou até cancelar).

### Autenticação
- THE SYSTEM SHALL exigir um JWT válido no metadata `authorization: Bearer <jwt>` (validado via JWKS do identity); senão -> `Unauthenticated`.
- THE SYSTEM SHALL identificar o jogador pelo `sub` do token, nunca por um id no request.

### Entrar na fila e acompanhar o ticket
- WHEN o cliente chama `FindMatch` (autenticado) -> cria um ticket para o jogador e emite `TicketUpdate{status: SEARCHING}`.
- WHILE o ticket aguarda -> o serviço mantém o stream aberto (sem polling do cliente).
- WHEN o jogador é agrupado num match -> emite `TicketUpdate{status: MATCHED, matchId, players[], connectInfo}` e **encerra** o stream.

### Cancelar / sair da fila
- WHEN o cliente cancela a chamada (fecha o stream / cancela o context) -> o ticket é **removido** da fila e não entra em nenhum match.
- THE SYSTEM SHALL garantir que um jogador cancelado nunca seja incluído num match formado depois.

### Formação de match
- THE SYSTEM SHALL formar um match assim que houver `matchSize` tickets ativos na fila, na ordem de chegada (FIFO).
- THE SYSTEM SHALL atribuir o **mesmo** `matchId` a todos os jogadores do match e notificá-los.
- THE SYSTEM SHALL evitar que o mesmo jogador (mesmo `sub`) ocupe duas vagas — um ticket por jogador.

## Contratos de Interface

### Protobuf (`Matchmaking`)
```
service Matchmaking {
  rpc FindMatch(FindMatchRequest) returns (stream TicketUpdate);
}
message FindMatchRequest {}            // vazio no MVP (skill entra na 002)
message TicketUpdate {
  Status status = 1;
  string match_id = 2;                 // preenchido em MATCHED
  repeated string players = 3;         // playerIDs do match
  string connect_info = 4;            // mock até o session-service existir
}
enum Status { SEARCHING = 0; MATCHED = 1; }
```

### Matchmaker (Port de entrada, domínio)
- `Join(ctx, playerID) (<-chan Match, error)` — enfileira o ticket; o `Match` chega pelo canal quando formado. Cancelar o `ctx` remove o ticket.
- `Run(ctx)` — loop de formação (executado em background).

### TokenVerifier (Port de saída)
- `Verify(ctx, token) (playerID string, error)` — `shared/auth` (JWKS), usado no interceptor gRPC.

### Tipos (domínio)
- `Match{ ID string; Players []string; ConnectInfo string }`

## Modelo de Estado

**Fila em memória** (sem storage externo no MVP)

| Item | Como |
|---|---|
| Fila de tickets | slice/map protegido por `sync.Mutex` |
| Ticket | `{ playerID, canal de Match }` |
| Formação | goroutine worker (ticker) que agrupa `matchSize` tickets e notifica os canais |
| Cancelamento | `ctx.Done()` do stream -> remove o ticket da fila |

## Casos de Teste Obrigatórios

- [ ] `matchSize` jogadores entram -> todos recebem um `Match` com o **mesmo** `matchId` e a lista de players
- [ ] Match formado tem exatamente `matchSize` jogadores
- [ ] Jogador que cancela (ctx) antes de formar -> sai da fila e não aparece em match
- [ ] Com menos de `matchSize` na fila -> ninguém é matched (fica SEARCHING)
- [ ] gRPC sem metadata de auth / token inválido -> `Unauthenticated`
- [ ] Dois `FindMatch` simultâneos do mesmo jogador não criam duas vagas

## Fora do escopo

- **Skill-based matching** (faixa de skill/MMR) — próxima feature
- **Agrupamento por região/latência** e **relaxamento progressivo de regras** — próximo
- **Party/grupos e backfill** — backlog
- **Chamada real ao session-service** (alocar server) — quando o session existir; MVP usa connect-info mock
- **Persistência/escala horizontal da fila** (Redis) — MVP é em memória, instância única
