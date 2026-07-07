# Spec: 007-summon-signs

**Serviço:** matchmaking-service
**Status:** Aprovada (2026-07-07)
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — **arquétipo `host_world_dropin`, fatia H1b** (co-op por quadro de sinais)
**Roadmap:** `docs/game-models/host_world_dropin-roadmap.md` → H1b
**Depende de:** `session/011` (`JoinWorld` com `role=coop`) e H1a (`matchmaking/006` — reusa a faixa/critério de elegibilidade).

## Contexto

A H1a deu ao matchmaking o **segundo modo** — invasão por **busca + injeção**: o invasor pede um alvo e o sistema o injeta num mundo elegível (o host **não** escolhe). É assimétrico num sentido: o invasor age, o host sofre.

O co-op (ficha 01, §4) é o **outro fluxo assimétrico**, de **dois passos**, e é a novidade desta fatia:

1. o **aliado anuncia** um sinal (disponibilidade para ajudar) num quadro, com a sua faixa (soul level / weapon tier), e **espera**;
2. o **host vê os sinais compatíveis** com o seu mundo e **escolhe** quem invocar.

Não é uma fila que resolve sozinha (queue_arena), nem uma injeção automática (invasão): é um **quadro de sinais** onde a decisão é do host. O aliado invocado é então **injetado** no mundo do host como `coop` (via `JoinWorld`, H0) e recebe o endereço do server + connect token.

Esta spec cobre o **lado matchmaking** do co-op: o store efêmero de sinais, a compatibilidade por faixa (reusando a política da H1a, mas **simétrica** para co-op), e a injeção na invocação. O resultado por evento/papel (o co-op "ajudou a matar o boss") é H2.

> **Escopo deliberadamente estreito:** o essencial do quadro de dois passos. **Senha** (play-with-friends, que ignora a faixa) e **pools/área/região** ficam como follow-up — a H1a também adiou a senha. Ver "Fora de escopo".

## Comportamento

### Anunciar um sinal (aliado, passo 1)

- WHEN um aliado chama `PostSummonSign(gameID, coopProfile{soulLevel, weaponTier})` para um jogo cujo perfil é `host_world_dropin` **com co-op habilitado** (papel `coop` em `EnabledRoles`) → THE SYSTEM SHALL registrar um **sinal** no quadro do jogo, com um `signID` próprio, o `playerID` do aliado (do JWT), a faixa informada, e um **TTL**; e devolver ao aliado uma confirmação de que o sinal está **publicado** (`POSTED`, com o `signID`).
- THE SYSTEM SHALL manter o aliado **aguardando a invocação** enquanto o sinal viver: `PostSummonSign` é **server-streaming** (espelha o `FindMatch`) — o aliado recebe `POSTED` e depois **bloqueia** até ser invocado (`SUMMONED`, com `sessionID` + `server_address` + `connect_token` do papel `coop`) ou o sinal **expirar/cancelar** (`EXPIRED`).
- WHEN o aliado **cancela** (fecha o stream / `ctx` cancelado) antes de ser invocado → THE SYSTEM SHALL **remover** o sinal do quadro (não fica lixo invocável). Simétrico ao ticket do `FindMatch` que sai da fila ao cancelar.
- WHEN o TTL do sinal expira sem invocação → THE SYSTEM SHALL removê-lo do quadro e encerrar o stream do aliado com `EXPIRED`.
- IF o perfil do jogo **não** é `host_world_dropin`, ou não tem `coop` habilitado → THE SYSTEM SHALL rejeitar com `FailedPrecondition` (**ErrCoopNotSupported**).

### Ver os sinais compatíveis (host, passo 2a)

- WHEN o host chama `ListSigns(gameID, hostSoulLevel, hostWeaponTier)` → THE SYSTEM SHALL devolver os **sinais ativos** daquele jogo **compatíveis** com o host: dentro da **faixa de co-op** (simétrica) e, se o perfil exigir, do **weapon tier** compatível. Cada item traz `signID`, `playerID` do aliado e a faixa dele.
- THE SYSTEM SHALL devolver **lista vazia** (não erro) quando nenhum sinal é compatível — quadro vazio é normal.
- WHEN o perfil tem `LevelRangeEnabled=false` → THE SYSTEM SHALL **não** filtrar por faixa (todo sinal ativo é compatível). `WeaponTierEnabled=false` → ignora weapon tier. Reusa exatamente as flags e o esqueleto de elegibilidade da H1a, com a **banda simétrica** do co-op.
- THE SYSTEM SHALL escopar o quadro por `gameID` (multi-tenant, ADR-0017): o host só vê sinais do próprio jogo.

### Invocar (host, passo 2b) → injeção

- WHEN o host chama `SummonSign(sessionID, signID)` para um sinal ativo → THE SYSTEM SHALL **injetar** o aliado do sinal no mundo `sessionID` como `coop` (`session.JoinWorld(sessionID, allyID, "coop")`, H0), **consumir** o sinal (some do quadro, não pode ser reinvocado) e **entregar** ao aliado — pelo stream aberto do `PostSummonSign` — o `sessionID` + `server_address` + `connect_token` (`SUMMONED`). Ao host, devolve uma confirmação (o mundo é dele; ele já está conectado).
- IF o `signID` **não existe** (nunca existiu, expirou, ou foi cancelado) → THE SYSTEM SHALL responder **ErrSignNotFound** (`NotFound`).
- IF o `signID` **já foi consumido** por uma invocação anterior (corrida entre dois hosts, ou clique duplo) → THE SYSTEM SHALL responder **ErrSignConsumed** (`FailedPrecondition`) — idempotência da invocação. O sinal é consumido **no máximo uma vez**.
- IF a injeção falha porque o mundo **não tem slot de co-op livre** (`JoinWorld` → `ErrRoleFull`) → THE SYSTEM SHALL **não consumir** o sinal (o aliado segue disponível para outro host) e responder `ResourceExhausted` (**ErrRoleFull**). Se o mundo **não existe/fechou** (`ErrWorldNotFound`) → `NotFound`.
- THE SYSTEM SHALL garantir que **um sinal casa com no máximo um host**: a transição "ativo → consumido" é atômica; a injeção só é tentada pelo host que ganhou a transição.

### Isolamento dos outros modos

- THE SYSTEM SHALL manter `FindMatch` (queue-form) e `Invade` (invasão, H1a) **inalterados**. O quadro de sinais é um terceiro caminho, independente: não usa a fila, não forma match, não injeta invasor.

## Contratos de Interface

### Protobuf (`matchmaking/v1/matchmaking.proto`)

- Novo RPC **server-streaming** `PostSummonSign(PostSummonSignRequest{ string game_id; int32 soul_level; int32 weapon_tier }) → stream SummonUpdate`.
  - `SummonUpdate{ SummonStatus status; string sign_id; string session_id; string server_address; string connect_token }` com `enum SummonStatus { UNSPECIFIED; POSTED; SUMMONED; EXPIRED }`.
- Novo RPC **unary** `ListSigns(ListSignsRequest{ string game_id; int32 host_soul_level; int32 host_weapon_tier }) → ListSignsResponse{ repeated SummonSign signs }` com `SummonSign{ string sign_id; string player_id; int32 soul_level; int32 weapon_tier }`.
- Novo RPC **unary** `SummonSign(SummonSignRequest{ string session_id; string sign_id }) → SummonSignResponse{ string player_id }` (o aliado invocado; a entrega do server/token vai pelo stream do aliado).
- O `playerID` (aliado no `PostSummonSign`; host no `ListSigns`/`SummonSign`) vem do **JWT** (interceptors — o unary já existe da H1a; o stream já existe do `FindMatch`).
- `FindMatch`/`Invade` — inalterados. Regenerar com `buf`.

### Ports (domínio)

- **Novo** `SummonBoard` (Port de saída, o store efêmero): `Post(sign) → (Sign, <-chan Summoned)`, `List(gameID) → []Sign`, `Consume(signID) → (Sign, error)` (transição atômica ativo→consumido; `ErrSignNotFound`/`ErrSignConsumed`), `Remove(signID)` (cancelamento/TTL), `Deliver(signID, Summoned)`. A implementação (em memória com TTL + canal por sinal) é decisão do plan.
- **Reusa** `WorldInjector` (H1a): `Inject(ctx, sessionID, allyID, "coop")`. O papel muda; o adapter é o mesmo.
- **Reusa** `InvasionPolicyProvider` (H1a) ou o perfil — precisa de `Archetype` + `coop` habilitado + `LevelRangeEnabled`/`WeaponTierEnabled`. Decisão do plan: estender a policy lida da H1a (ela já traz `LevelRangeEnabled`/`WeaponTierEnabled`) com um flag/lista de papéis, ou uma leitura irmã.
- **Novo** serviço de aplicação (ex.: `Summoner` / método no serviço) — orquestra post→espera / list→filter / summon→consume→inject.
- **Novo** critério de faixa **simétrica** de co-op: função de domínio pura (irmã de `Eligible`), `CoopEligible(ally, host, policy) bool`.

### Erros de domínio (novos)

- `ErrCoopNotSupported` → `FailedPrecondition` (arquétipo errado ou `coop` desabilitado).
- `ErrSignNotFound` → `NotFound`; `ErrSignConsumed` → `FailedPrecondition`.
- Reusa `ErrRoleFull`/`ErrWorldNotFound` (do injetor) e `ErrProfileUnavailable`.

## Modelo de Dados

- **Sinal (`Sign`)**: `signID`, `gameID`, `playerID` (aliado), `soulLevel`, `weaponTier`, `postedAt`, mais um **canal de entrega** (`chan Summoned`) para acordar o stream do aliado na invocação — espelha o `ticket.ch` do `FindMatch`.
- **Store efêmero, com TTL.** Recomendação (a confirmar no plan): **em memória**, protegido por mutex, com um **sweeper de TTL** (mesmo padrão do `Sweep` do session-service). Motivo forte para memória nesta fatia: a entrega ao aliado é por **canal Go**, que não atravessa Redis (multi-instância exigiria pub/sub — evolução declarada). Sem persistência: sinal some no restart (aceitável — é presença efêmera).
- **Idempotência da invocação:** a chave é o `signID`; a transição ativo→consumido é atômica sob o lock. Não há chave externa tipo `matchID`.

## Observabilidade

- `summon_signs_active{game}` (gauge) — sinais no quadro por jogo (funil de co-op no dashboard, H3).
- `summons_total{result="summoned|expired|cancelled"}` (counter) — desfecho dos sinais.
- Tracing: os 3 RPCs no otelgrpc global; a injeção herda o span via otelgrpc client.

## Fora de escopo

- **Senha (play-with-friends):** o `PasswordEnabled` existe no perfil; o casamento por senha (que **ignora** a faixa) é **follow-up**, como na H1a. Esta fatia casa co-op **por faixa**.
- **Pools / área / região:** o sinal da ficha vive num pool ligado a uma área do mapa. Nesta fatia o quadro é **por jogo** (sem dimensão de área). Pools = follow-up.
- **Resultado do co-op** (ajudou a matar o boss → recompensa) → H2 (eventos de participação).
- **Store distribuído (Redis + pub/sub)** para multi-instância — evolução declarada; memória no MVP.
- **MMR/skill** na compatibilidade — casual (a ficha diz "sem MMR").
- **Data plane** que encarna o mundo e lê o `role=coop` do token — cross-repo (planejado).

## Critérios de Aceite

- [ ] `PostSummonSign` num jogo `host_world_dropin` com co-op ligado → publica o sinal (`POSTED` + `signID`) e mantém o stream aberto; cancelar o stream remove o sinal.
- [ ] Jogo não-`host_world_dropin` ou sem `coop` → `ErrCoopNotSupported` (`FailedPrecondition`).
- [ ] `ListSigns` devolve só os sinais do jogo **compatíveis** com o host (faixa **simétrica** de co-op; weapon tier se ligado); `LevelRangeEnabled=false` → todos; nenhum compatível → lista vazia.
- [ ] `SummonSign` de um sinal ativo → injeta o aliado como `coop` (`JoinWorld`), consome o sinal, e o aliado recebe `SUMMONED` (server + token) pelo stream; some do `ListSigns` seguinte.
- [ ] Invocar um sinal inexistente → `ErrSignNotFound`; invocar um já consumido → `ErrSignConsumed` (consumo no máximo uma vez, mesmo sob corrida).
- [ ] Mundo sem slot de co-op livre → `ResourceExhausted` e o sinal **não** é consumido (segue disponível).
- [ ] TTL expira → o aliado recebe `EXPIRED` e o sinal some do quadro.
- [ ] `CoopEligible` (banda simétrica) testada isolada; `FindMatch`/`Invade` sem regressão; testes com `-race`, ≥80% em `service/`/`domain/`.
