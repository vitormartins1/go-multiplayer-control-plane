# matchmaking-service

≈ **GameLift FlexMatch**. **Primeiro serviço gRPC** (ADR-0003): o jogador entra na fila e acompanha o ticket por **server-streaming** até a partida formar (FIFO por tamanho de time). É o "core de concorrência" do backend (fila + worker + channels).

- **Protocolo:** gRPC (server-streaming) · **Portas host:** 50051 (gRPC), 8086 (HTTP: `/metrics`, `/healthz`)
- **Storage:** memória (fila com `sync.Mutex`; prod usaria Redis)
- **Dependências:** identity (JWT via interceptor gRPC)

## Contrato (protobuf — `proto/matchmaking/v1`)

```
service Matchmaking {
  rpc FindMatch(FindMatchRequest) returns (stream TicketUpdate); // SEARCHING → MATCHED
}
```
Código gerado por **buf** (`buf generate`), commitado em `gen/`. Reflection habilitada (grpcurl sem o `.proto`).

## Modelo de dados

**Estado em memória** — sem persistência. A fila é um slice de **tickets** (`{ playerID, channel de Match }`) protegido por `sync.Mutex`; o worker forma matches FIFO e entrega cada `Match{ ID, Players, ConnectInfo }` pelo channel do ticket. Estado efêmero (some no restart); escala horizontal exigiria fila compartilhada (Redis).

## Observabilidade

Métricas custom em `/metrics` (porta 8086): `matchmaking_queue_size`, `matchmaking_matches_total` + runtime Go. Tracing via **otelgrpc**. Aparece no **service graph** (edge `matchmaking → identity` do fetch de JWKS).

## Testar / Specs

- Manual (gRPC): [`api/README.md`](api/README.md) — exemplos com `grpcurl`
- Specs: [`specs/`](specs/) — `001-matchmaking-core`
