# Spec: 012-invadable-world-directory

**Serviço:** session-service
**Status:** Done
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo) — **arquétipo `host_world_dropin`, fatia H1a** (lado session: atributos do host + diretório de mundos)
**Roadmap:** `docs/game-models/host_world_dropin-roadmap.md` → H1a

## Contexto

A H0 (`session/011`) tornou o mundo host-cêntrico real: `OpenWorld` abre uma sessão persistente com slots por papel, e `JoinWorld` injeta um phantom. Mas o `OpenWorld` guarda o **mínimo** (host + jogo) — o mundo **não sabe descrever a que faixa pertence** nem há como **listar mundos abertos**.

A H1a (matchmaking por injeção/invasão, spec `matchmaking/006`) precisa de duas coisas do session que a H0 não entregou:

1. **Atributos de matchmaking do host** — para casar um invasor por faixa, o mundo precisa carregar o **soul level** e o **weapon tier** do host (ficha 01, §4: o "range" da invasão é calculado sobre o nível). Hoje o `OpenWorld` os descarta.
2. **Um diretório de mundos invadíveis** — o matchmaking descobre alvos consultando o session (modelo **pull**, roadmap H1a): "quais mundos deste jogo estão abertos com **slot de invasor livre**, e quais os atributos deles?".

Esta spec entrega os dois no session-service. A **política** (quem casa com quem, a matemática da faixa, se o jogo permite invasão) mora no matchmaking (`006`) — aqui o session é uma **fonte de dados pura**: guarda os atributos e lista os mundos, sem decidir elegibilidade.

> **Escopo deliberadamente estreito:** só o que a descoberta por injeção precisa. Sinais de co-op (H1b) e eventos de participação (H2) são outras fatias.

## Comportamento

### Atributos de matchmaking no `OpenWorld`

- WHEN o host chama `OpenWorld` informando **soul level** e/ou **weapon tier** → THE SYSTEM SHALL guardá-los na sessão do mundo, para o matchmaking casar por faixa depois.
- WHEN o host chama `OpenWorld` **sem** esses atributos (ausentes/zero) → THE SYSTEM SHALL abrir o mundo normalmente; o mundo fica com atributos zerados (o matchmaking o tratará como não-casável por faixa, decisão da spec `006`). Nenhuma regressão sobre a H0.
- THE SYSTEM SHALL manter todo o resto do `OpenWorld` (H0) inalterado: alocação, slots por papel, token do host com claim `role=host`.

### Diretório de mundos invadíveis

- WHEN o matchmaking chama `ListInvadableWorlds(gameID)` → THE SYSTEM SHALL devolver os mundos **abertos** (`Origin=host_world`, `State=open`) **daquele jogo** que têm um **slot de invasor com capacidade livre**, cada um com: `sessionID`, `hostSoulLevel`, `weaponTier` e `freeInvaderSlots`.
- THE SYSTEM SHALL **excluir** da lista: mundos de outros jogos; mundos sem papel `invader` habilitado (sem slot de invasor); mundos com o slot de invasor **cheio**; sessões `queue_arena` (`Origin=match`); sessões fechadas.
- THE SYSTEM SHALL devolver uma **lista vazia** (não erro) quando nenhum mundo é elegível — a ausência de alvo é um resultado normal, não uma falha.
- THE SYSTEM SHALL **não** aplicar filtro de faixa, weapon tier ou senha — isso é política do matchmaking (`006`). O diretório é uma fotografia dos mundos abertos com vaga de invasor.

### Consistência com o `JoinWorld` (H0)

- THE SYSTEM SHALL manter o `JoinWorld` como o ponto de injeção: o matchmaking, após escolher um mundo do diretório, chama `JoinWorld(sessionID, invaderID, "invader")` (H0, inalterado). Entre listar e injetar há uma corrida natural (o slot pode encher) — o `JoinWorld` já a resolve com `ErrRoleFull`; esta spec **não** reserva slot na listagem.

### Compatibilidade

- `queue_arena` (specs 001–010) e o `host_world` da H0 seguem sem mudança observável. Os campos de atributo no `OpenWorld` e o novo `ListInvadableWorlds` são **aditivos**.

## Contratos de Interface

### Protobuf (`session/v1/session.proto`)

- `OpenWorldRequest` ganha campos **opcionais**: `int32 host_soul_level`, `int32 weapon_tier`. Ausentes = 0 (mundo sem faixa). `OpenWorldResponse` inalterado.
- Novo RPC `ListInvadableWorlds(ListInvadableWorldsRequest{ string game_id }) → ListInvadableWorldsResponse{ repeated InvadableWorld worlds }` com `InvadableWorld{ string session_id; int32 host_soul_level; int32 weapon_tier; int32 free_invader_slots }`.
- Regenerar com `buf` (nunca `sed`).

### Ports (domínio)

- **`Session`** ganha `HostSoulLevel int`, `WeaponTier int` (zero em `queue_arena` e em mundos sem atributos).
- **`SessionService.OpenWorld`** ganha os dois atributos na assinatura (ex.: um struct `WorldAttributes{SoulLevel, WeaponTier int}` para não crescer a lista posicional a cada evolução; decisão do plan).
- **Novo** `SessionService.ListInvadableWorlds(ctx, gameID) ([]InvadableWorld, error)` + tipo `InvadableWorld{ SessionID string; HostSoulLevel, WeaponTier, FreeInvaderSlots int }`.
- `ServerAllocator`, `ConnectTokenIssuer`, `ProfileProvider`, `ResultPublisher` — **inalterados**.

### Erros de domínio

- Nenhum novo. `ListInvadableWorlds` nunca erra por "vazio" (devolve lista vazia). Erros de infra seguem o padrão (wrap + `Internal` no transport).

## Modelo de Dados

Sessão continua **em memória** (H0). Os atributos entram na mesma struct `Session`; o diretório é derivado varrendo o mapa de sessões ativas sob o lock (como `AllSessions`/`SessionsByGame`). **Sem migration.**

## Observabilidade

- Métrica opcional `invadable_worlds{game}` (gauge) — quantos mundos com vaga de invasor por jogo. Barato e alimenta o funil de invasão no dashboard (H3). *(Pode ficar para H3; decidir no plan.)*
- Tracing: `ListInvadableWorlds`/`OpenWorld` já entram no otelgrpc global.

## Fora de escopo

- **Senha (play-with-friends):** a flag `PasswordEnabled` existe no perfil, mas o casamento por senha (que ignora faixa) é **follow-up** — H1a foca a invasão por faixa. O `OpenWorld` **não** recebe senha nesta fatia.
- **Política de elegibilidade** (faixa assimétrica, weapon tier, `InvasionEnabled`) — mora no matchmaking (`006`).
- **Sinais de co-op** (quadro de sinais) — H1b.
- **Reservar slot na listagem** — não; a corrida listar→injetar é resolvida pelo `JoinWorld` (`ErrRoleFull`).
- **Persistência da sessão / pools por área/região** — futuro.

## Critérios de Aceite

- [ ] `OpenWorld` com soul level + weapon tier guarda os dois na sessão; sem eles, abre igual (zeros).
- [ ] `ListInvadableWorlds(g)` devolve só mundos abertos de `g` com slot de invasor livre, com atributos e `freeInvaderSlots` corretos.
- [ ] Mundo com invasor cheio, sem papel invasor, de outro jogo, ou `queue_arena` → **fora** da lista.
- [ ] Nenhum mundo elegível → lista vazia, sem erro.
- [ ] `queue_arena` e H0 sem regressão; testes com `-race`, ≥80% em `service/`/`domain/`.
