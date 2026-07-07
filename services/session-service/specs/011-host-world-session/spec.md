# Spec: 011-host-world-session

**Serviço:** session-service
**Status:** Aprovada (2026-07-07)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — **arquétipo `host_world_dropin`, fatia H0** (sessão host-cêntrica persistente)
**Roadmap:** `docs/game-models/host_world_dropin-roadmap.md` → H0

## Contexto

O arquétipo `host_world_dropin` (ficha [01 — Soulslike](../../../../docs/game-models/01-soulslike-invasion-coop.md)) já está **declarado** no game-registry (`HostWorldDropInParams`: `EnabledRoles` `coop`/`invader`, `CoopSlots`, `InvaderSlots`, flags de faixa/senha/invasão) e no console. Mas **nenhum serviço reage a ele** — o session-service continua preso ao modelo do `queue_arena`: a sessão **nasce de um match**, N jogadores entram juntos, um `ReportResult` terminal encerra tudo.

Este arquétipo quebra esse modelo no eixo do **ciclo de vida da sessão**:

- A sessão **não nasce de um match** — ela é **o mundo de um host** ficando aberto ao online. O host abre; ninguém foi pareado ainda.
- Phantoms **entram e saem no meio** (join-in-progress), cada um ocupando um **slot de papel** (`coop` aliado / `invader` adversário), com **capacidade dinâmica** derivada do perfil.
- O mundo é **persistente**: um phantom sair **não** encerra a sessão (libera o slot). Só o **host sair** encerra a participação de todos.

Esta spec (H0) entrega **só o pivô**: a sessão host-cêntrica com slots por papel e join/leave. Ela **não** cobre como o phantom é descoberto/injetado (matchmaking — H1) nem o resultado por evento (H2) nem o console (H3). O ponto de entrada aqui é o próprio host (via control plane) e um **cliente de teste** que simula o phantom chamando `JoinWorld` como o matchmaking fará em H1.

> **Princípio (control plane primeiro):** não há data plane `host_world_dropin` (o `go-game-host-world-dropin-dataplane` Unity é ⬜ planejado — `docs/REPO-TOPOLOGY.md`). Esta fatia se valida com o `ServerAllocator` `static` (server mockado) e cliente de teste, como o `queue_arena` se validou antes do Unity. O server real que encarna o mundo é etapa cross-repo posterior.

## Comportamento

### Abrir o mundo (host)

- WHEN o host chama `OpenWorld(hostPlayerID, gameID)` para um jogo cujo perfil é `host_world_dropin` → THE SYSTEM SHALL alocar um game server, criar uma **sessão persistente** dona-por-host, derivar a **capacidade por papel** do perfil (`CoopSlots`/`InvaderSlots`, só para os papéis em `EnabledRoles`) e devolver o **connect token do host** (papel `host`) + o endereço do server.
- WHEN o perfil do jogo **não** é `host_world_dropin` → THE SYSTEM SHALL rejeitar com `FailedPrecondition` (**ErrUnsupportedArchetype**). Este arquétipo é o único que abre mundo sem match; `queue_arena` segue usando `CreateSession`.
- IF o pool de servers esgotar → THE SYSTEM SHALL rejeitar com `ResourceExhausted` (**ErrNoServer**), como no `CreateSession` atual.
- THE SYSTEM SHALL registrar a sessão com `Origin = host_world`, `State = open`, o `hostPlayerID`, e os slots por papel com `Occupants` inicialmente vazio (o host **não** ocupa slot de `coop`/`invader` — é o dono do mundo).

### Entrar no mundo (phantom, por papel)

- WHEN um phantom chama `JoinWorld(sessionID, playerID, role)` com `role ∈ {coop, invader}` habilitado no perfil e com **slot livre** → THE SYSTEM SHALL ocupar um slot daquele papel e devolver o **connect token** do phantom com o claim **`role`** (o server lê no *connection approval*, como já lê `team_id` no `queue_arena`).
- IF o `role` **não** está em `EnabledRoles` do jogo → THE SYSTEM SHALL rejeitar com `InvalidArgument` (**ErrRoleNotEnabled**).
- IF o papel está **cheio** (`Occupants == Capacity`) → THE SYSTEM SHALL rejeitar com `ResourceExhausted` (**ErrRoleFull**).
- IF a sessão não existe ou já está `closed` → THE SYSTEM SHALL rejeitar com `NotFound` (**ErrWorldNotFound**).
- IF o `playerID` já ocupa um slot naquele mundo → THE SYSTEM SHALL rejeitar com `FailedPrecondition` (**ErrAlreadyJoined**) — join idempotente-por-erro, não duplica slot.

### Sair do mundo (phantom) e encerrar (host)

- WHEN um phantom chama `LeaveWorld(sessionID, playerID)` → THE SYSTEM SHALL liberar o slot do papel dele e **manter a sessão aberta** (o mundo do host segue). O slot volta a ficar disponível para novos joins.
- WHEN o **host** sai (`CloseWorld(sessionID)` ou o host chama `LeaveWorld` sendo o dono) → THE SYSTEM SHALL encerrar a sessão com `CloseReason = host_left`, **liberar o server** e encerrar a participação de **todos** os phantoms (o data plane os retorna aos próprios mundos — cross-repo).
- IF `LeaveWorld` referencia um `playerID` que não ocupa slot naquele mundo → THE SYSTEM SHALL responder de forma **idempotente** (no-op, sucesso), não erro — leave é seguro de repetir.

### Reconciliação e TTL (herdado, adaptado)

- THE SYSTEM SHALL manter o mundo sujeito ao **sweeper de reconciliação** (spec 008, ADR-0022): se o game server do mundo sumir da fleet (crash) além do grace period → fechar com `CloseReason = abandoned`. O `host_left` é o encerramento **limpo**; `abandoned` é o backstop de crash.
- THE SYSTEM SHALL manter o mundo sujeito ao **TTL sweeper** como backstop final (mundo aberto além de `SESSION_TTL` sem atividade → `CloseTTL`). *(O ciclo de vida rico — fechar por inatividade real, não só idade — pode ser refinado em fatia posterior.)*

### Compatibilidade

- `CreateSession`/`ReportResult` do `queue_arena` (specs 001–010) seguem **inalterados**. `OpenWorld`/`JoinWorld`/`LeaveWorld`/`CloseWorld` são operações **novas**, paralelas.
- A sessão ganha `Origin` (`match` | `host_world`); sessões existentes/`queue_arena` são `match`. O `matchID` continua a chave do `queue_arena`; o `host_world` usa o `sessionID` como identidade (não há match).

## Contratos de Interface

### Protobuf (`session/v1/session.proto`)

Novos RPCs (aditivos, não tocam os existentes):

- `OpenWorld(OpenWorldRequest{ host_player_id, game_id }) → OpenWorldResponse{ session_id, server_address, ConnectGrant host_grant }`
- `JoinWorld(JoinWorldRequest{ session_id, player_id, string role }) → JoinWorldResponse{ ConnectGrant grant }`
- `LeaveWorld(LeaveWorldRequest{ session_id, player_id }) → LeaveWorldResponse{}` (vazio; idempotente)
- `CloseWorld(CloseWorldRequest{ session_id }) → CloseWorldResponse{}`

`ConnectGrant` é reusado (já traz `player_id` + `token`). O `role` viaja **no claim** do token, não em campo separado da resposta. `CreateSessionRequest`/`ReportResultRequest` — **inalterados**.

### Ports (domínio)

- **`SessionService`** ganha os quatro métodos: `OpenWorld(ctx, hostPlayerID, gameID) (*Session, ConnectGrant, error)`, `JoinWorld(ctx, sessionID, playerID, role) (ConnectGrant, error)`, `LeaveWorld(ctx, sessionID, playerID) error`, `CloseWorld(ctx, sessionID) error`. `Create`/`ReportResult` inalterados.
- **`Session`** (agregado) ganha: `Origin` (`match`|`host_world`), `HostPlayerID`, e slots por papel — ex. `RoleSlots map[string]RoleSlot{ Capacity int; Occupants []string }`. Forma exata (mapa vs. lista de `Participant{PlayerID, Role}`) = decisão do plan.
- **Nova Port de saída `ProfileProvider`** — o session precisa ler o perfil `host_world_dropin` do game-registry (arquétipo + `EnabledRoles`/`CoopSlots`/`InvaderSlots`). O matchmaking já tem uma `ProfileProvider` análoga (adapter gRPC/HTTP ao game-registry) — espelhar. Devolve `ErrProfileUnavailable` quando indisponível.
- **`ConnectTokenIssuer.Issue`** — precisa emitir o claim **`role`**. Hoje é `Issue(playerID, matchID, serverAddr, teamID)`. Decisão do plan: generalizar (ex. `Issue(playerID, sessionID/matchID, serverAddr, claims{team_id?, role?})`) ou sobrecarga. Recomendação: um struct de claims opcionais, para não crescer a assinatura a cada arquétipo. JWKS inalterado.
- **`ServerAllocator`**, **`ServerRegistry`**, **`ResultPublisher`** — **inalterados** (o fan-out de resultado é H2).

### Erros de domínio (novos)

- `ErrUnsupportedArchetype` → `FailedPrecondition` (jogo não é `host_world_dropin`).
- `ErrRoleNotEnabled` → `InvalidArgument` (papel fora de `EnabledRoles`).
- `ErrRoleFull` → `ResourceExhausted` (slots do papel esgotados).
- `ErrWorldNotFound` → `NotFound` (sessão inexistente/fechada).
- `ErrAlreadyJoined` → `FailedPrecondition` (jogador já no mundo).
- Reusa `ErrNoServer` (pool esgotado) e `ErrProfileUnavailable`.

Novo `CloseReason` **`host_left`** (encerramento limpo pelo host), distinto de `result`/`ttl`/`admin`/`abandoned`.

## Modelo de Dados

Sessão continua **em memória** (spec 001/008: `s.sessions` map) — mundos host-cêntricos vivem na mesma estrutura, com os campos novos (`Origin`, `HostPlayerID`, `RoleSlots`). **Sem migration.** Persistência da sessão segue fora de escopo (como no `queue_arena`).

## Observabilidade

- Métrica `sessions_opened_total{origin}` (ou reuso da contagem existente com label `origin`) distinguindo `host_world` de `match`.
- Gauge de **slots ocupados por papel** por mundo aberto (`world_role_slots{role}`), para o Rooms (H3) e o dashboard `game-lifecycle`.
- `sessions_closed_total{reason="host_left"}` cai no painel "Sessões encerradas/s por motivo" já existente (pega o label sozinho).
- Tracing: `OpenWorld`/`JoinWorld` instrumentados (OTel), como os RPCs atuais.

## Fora de escopo (fatias seguintes / cross-repo)

- **Descoberta/injeção do phantom** (busca por faixa, quadro de sinais) → matchmaking, **H1a/H1b**. Aqui o `JoinWorld` é chamado direto (host/cliente de teste); em H1 o matchmaking o chama após casar faixa/senha/papel.
- **Resultado por evento/papel** (`ParticipationEvent`, fan-out) → **H2**. Esta fatia não reporta resultado — o mundo abre, phantoms entram/saem, o host encerra.
- **Surfacing no console** (Rooms com papéis/slots) → **H3**.
- **Alocação sob demanda** (alocar server só quando o mundo abre para multiplayer, liberar ao esvaziar — a economia real): esta fatia aloca no `OpenWorld` (decisão #2 do roadmap, opção **a**). A opção **b** é evolução declarada, candidata a ADR.
- **Topologia `p2p`** (host como listen-server): dedicated primeiro; `p2p` é slot no perfil, ADR futuro.
- **Data plane Unity** que encarna o mundo e lê o claim `role`: cross-repo, `go-game-host-world-dropin-dataplane` (⬜ planejado). Aqui só se emite o claim.

## Critérios de Aceite

- [ ] `OpenWorld` num jogo `host_world_dropin` aloca server, cria sessão `Origin=host_world` com slots derivados do perfil, e devolve o connect token do host (claim `role=host`).
- [ ] `OpenWorld` num jogo `queue_arena` (ou sem perfil) → `ErrUnsupportedArchetype` (`FailedPrecondition`).
- [ ] `JoinWorld` com papel habilitado e slot livre ocupa o slot e emite token com claim `role`.
- [ ] `JoinWorld` com papel fora de `EnabledRoles` → `ErrRoleNotEnabled`; com papel cheio → `ErrRoleFull`; em mundo inexistente/fechado → `ErrWorldNotFound`; jogador já presente → `ErrAlreadyJoined`.
- [ ] `LeaveWorld` de phantom libera o slot e mantém a sessão aberta; repetir o leave é no-op (idempotente).
- [ ] Host sair (`CloseWorld`/`LeaveWorld` do dono) encerra com `CloseReason=host_left` e libera o server.
- [ ] O sweeper de reconciliação (008) fecha um mundo cujo server sumiu como `abandoned`; o TTL sweeper segue como backstop.
- [ ] `queue_arena` (`CreateSession`/`ReportResult`) sem regressão.
- [ ] Testes com `-race`, ≥80% em `service/`/`domain/`; cliente de teste simula o phantom (`JoinWorld`/`LeaveWorld`).

## Nota de implementação (possível sub-fatiamento)

Se a branch crescer demais, quebrar em **H0a** (open/close + slots por papel, token do host sem `role`) e **H0b** (join/leave por papel + claim `role` no token). Decisão no `plan.md`.
