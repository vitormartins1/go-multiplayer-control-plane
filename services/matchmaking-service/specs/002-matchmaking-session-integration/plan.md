# Plan: 002-matchmaking-session-integration
**Spec:** specs/002-matchmaking-session-integration/spec.md
**Data:** 2026-06-29

## Abordagem técnica

O matchmaking vira **cliente gRPC** do session (ADR-0003), mantendo a arquitetura hexagonal (ADR-0002): a chamada ao session entra como uma **Port de saída** `SessionProvisioner` no `domain`, com um **adapter** que fala o gRPC do session. O `service` (matchmaker) deixa de cunhar `connect_info` mock e passa a **provisionar a sessão** ao formar o match.

A formação continua FIFO em memória sob `sync.Mutex` (001). A novidade é que o resultado entregue a cada ticket passa a ser **por jogador** (cada um recebe o seu connect token), então o canal do ticket deixa de carregar `domain.Match` e passa a carregar um **resultado por jogador** (sucesso com token próprio, ou falha com motivo). O `tryForm` agrupa os tickets sob o lock (rápido) e dispara **uma goroutine por match** para chamar `CreateSession` e entregar os resultados — assim um `CreateSession` lento não bloqueia a fila (decisão da spec).

Reuso o **client gerado do session** (`services/session-service/gen/session/v1`) diretamente — é monorepo de módulo único (ADR-0001), o pacote gerado já está commitado. O matchmaking conecta no session via `SESSION_GRPC_ADDR` lido direto do ambiente no `main` (sem tocar em `shared/config`, igual ao `SERVER_POOL` do session).

## Arquivos a criar ou modificar

**Proto / codegen**
- `proto/matchmaking/v1/matchmaking.proto` — `TicketUpdate`: remove `connect_info`, adiciona `server_address`, `connect_token`, `reason`; `Status`: adiciona `STATUS_FAILED`. `buf generate` → regenera `gen/matchmaking/v1/*.pb.go`.

**Domain**
- `internal/domain/matchmaking.go` —
  - novo Port `SessionProvisioner` (saída): `Provision(ctx, matchID, players) (Provisioned, error)`
  - novo tipo `Provisioned{ ServerAddress string; Tokens map[string]string }` (token por playerID)
  - novo tipo `MatchResult` entregue por ticket: `{ MatchID string; Players []string; ServerAddress, ConnectToken string; Failed bool; Reason string }`
  - `Matchmaker.Join` muda o retorno de `<-chan Match` para `<-chan MatchResult`
  - remove `Match.ConnectInfo` (ou aposenta `Match`, ver decisões)
- `internal/domain/errors.go` (novo) — `ErrNoServer` (mapeado de `ResourceExhausted` do session)

**Service**
- `internal/service/matchmaker.go` —
  - `New` recebe o `SessionProvisioner` como dependência
  - `ticket.ch` passa a `chan domain.MatchResult` (buffer 1, mantém "tryForm nunca bloqueia")
  - `tryForm` (sob lock): agrupa `matchSize` tickets + gera `matchID`; **fora do lock** dispara `go m.provision(ctx, matchID, group)`
  - `provision`: chama `SessionProvisioner.Provision`; em sucesso, envia a cada ticket um `MatchResult` com o token do **seu** player; em falha (`ErrNoServer` ou qualquer erro) envia `MatchResult{Failed:true, Reason}`; fecha os canais
  - `tryForm` precisa do `ctx` do `Run` → guardar/propagar (passar `ctx` no `tryForm`); a goroutine usa `context.WithTimeout` derivado para a chamada gRPC
  - contadores de métrica: `matches_formed` (sucesso) + novo `matches_failed`
- `internal/service/matchmaker_test.go` — atualizar os testes da 001 (não há mais `ConnectInfo`); adicionar mock de `SessionProvisioner`; cobrir: sucesso entrega token por jogador, `ErrNoServer`→FAILED, erro genérico→FAILED, re-join após falha

**Adapter (cliente do session)**
- `internal/sessionclient/grpc_client.go` (novo) — implementa `domain.SessionProvisioner` sobre `sessionv1.SessionClient`; mapeia a resposta (`grants[]`) para `Provisioned{ServerAddress, Tokens}`; mapeia `codes.ResourceExhausted` → `domain.ErrNoServer`
- `internal/sessionclient/grpc_client_test.go` (novo) — bufconn subindo o **session real in-process** (allocator + token + publisher do session-service) para validar o mapeamento ponta a ponta (sucesso e pool esgotado → `ErrNoServer`)

**Transport**
- `internal/transport/grpc_server.go` — `FindMatch` consome `MatchResult`: `Failed` → `TicketUpdate{STATUS_FAILED, reason}`; sucesso → `TicketUpdate{STATUS_MATCHED, matchId, players, server_address, connect_token}`
- `internal/transport/grpc_server_test.go` — fake `SessionProvisioner`; assert MATCHED traz `server_address` + `connect_token` corretos por jogador; caso `ErrNoServer` → `STATUS_FAILED`

**Wire-up / infra**
- `cmd/server/main.go` — lê `SESSION_GRPC_ADDR`; `grpc.NewClient` para o session (insecure, otelgrpc client handler para propagar o trace); cria `sessionclient` adapter; injeta no `service.New`; fecha a conn no shutdown
- `services/matchmaking-service/api/README.md` — atualizar o exemplo do MATCHED (agora `server_address` + `connect_token`; explicar o FAILED)
- `infra/docker/docker-compose.yml` — `matchmaking-service`: `SESSION_GRPC_ADDR: session-service:50051` + `depends_on: session-service`

## Compute e banco (ver ADR-0005)
- Sem mudança: matchmaking segue sem banco (fila em memória). Nenhuma nova dependência de storage. O session é dono do estado da sessão.

## Dependências e libs
- Nenhuma nova externa. Usa `google.golang.org/grpc` (já presente), o client gerado do session (`services/session-service/gen/session/v1`) e `otelgrpc` (já presente) para o trace cross-service.

## Decisões técnicas e trade-offs

- **Importar o client gerado do session** (vs. proto compartilhado): no monorepo de módulo único o `gen` do session já está commitado e importável; é o caminho mais simples. Trade-off: acopla o matchmaking ao pacote gerado do session — aceitável porque é control plane interno e mesmo repo. Um módulo proto compartilhado só se justifica quando houver muitos consumidores.
- **Goroutine por match para o `CreateSession`** (decisão da spec): mantém o loop de formação responsivo; o canal do ticket é buffered (1), então o envio nunca bloqueia mesmo se o cliente já saiu. Risco: provisionamentos concorrentes — cobrir com `-race`.
- **Falha de provisionamento encerra o stream** (`STATUS_FAILED`, decisão da spec): sem re-enfileiramento nem retry/backoff no MVP. Mais simples; resiliência fica para backlog.
- **Sem auth serviço-a-serviço** matchmaking→session (control plane interno, alinhado à spec do session). **Provável ADR futuro** quando entrar service-to-service auth (mesma discussão do fan-out do session) — não criar ADR agora.
- **Trace cross-service:** usar o otelgrpc client handler na conn para o session, propagando o trace do FindMatch até o CreateSession (ADR-0014) — fecha a visão end-to-end no Tempo.
- **`SESSION_GRPC_ADDR` lido no `main`** (não em `shared/config`): evita mexer no pacote compartilhado para uma env de um serviço só (mesmo padrão do `SERVER_POOL`).

## Ordem de implementação

1. **proto** + `buf generate` (novos campos + `STATUS_FAILED`); `go build` do gerado
2. **domain**: `SessionProvisioner`, `Provisioned`, `MatchResult`, `ErrNoServer`; ajustar `Join`/`Match`
3. **sessionclient** (adapter) + test bufconn (session in-process: sucesso + `ErrNoServer`)
4. **service**: injeção do provisioner, `tryForm`→goroutine `provision`, canal `MatchResult`; atualizar/expandir testes (`-race`)
5. **transport**: mapear `MatchResult`→`TicketUpdate` (MATCHED/FAILED); atualizar testes bufconn
6. **wire-up** `main` (dial session + otelgrpc + shutdown) + compose + api/README
7. `go test ./... -race` verde; `go build ./...`; validar no Docker: FindMatch (2 clientes) → MATCHED com `server_address` + `connect_token` verificável no JWKS do session; derrubar/saturar o session → `STATUS_FAILED`

## Riscos e pontos de atenção

- **Mudança incompatível no proto** (remove `connect_info`, renumera): ninguém externo consome ainda; o matchmaking é o único produtor. Regenerar o `gen` e atualizar os testes da 001.
- **Lifecycle da goroutine de provisionamento:** respeitar o `ctx` do `Run` (shutdown) e usar timeout na chamada gRPC; canal buffered para não vazar goroutine se o cliente sumiu.
- **Concorrência** (formação + N provisionamentos): rodar tudo com `-race`.
- **Ordem de boot no compose:** matchmaking depende do session no ar; `depends_on` cobre o start, mas a chamada deve degradar para `STATUS_FAILED` (não crashar) se o session estiver indisponível — exatamente o comportamento da spec.
- **grpcurl ausente local:** validar via cliente Go temporário (como no 001 e no session).
