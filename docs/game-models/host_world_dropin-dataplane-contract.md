# Contrato control ↔ data plane — arquétipo `host_world_dropin`

> **Escopo:** o contrato entre o **control plane** (`go-game-backend`, Go) e o **data plane** (server do mundo, Unity) para o arquétipo **`host_world_dropin`** do ADR-0023 — o mundo persistente de um host onde phantoms **co-op** e **invasor** entram/saem no meio, com **papéis assimétricos** e **resultado por evento/participante** (não um placar terminal).
>
> Este doc é a **fonte de verdade do contrato**. O data plane (`go-game-host-world-dropin-dataplane`, Unity) está **⬜ planejado** ([REPO-TOPOLOGY.md](../REPO-TOPOLOGY.md)) — este documento é o que esse repo vai consumir. O control plane já está pronto e testado isoladamente com server mockado (adapter `static` + cliente de teste).
>
> **Refs:** ADR-0023 (perfil de multiplayer), ADR-0010 (control/data plane), ADR-0011/0012 (Agones/Unity), ADR-0020 (fan-out). Specs: `session/011` (mundo host-cêntrico), `session/012` (diretório de invadíveis), `session/013` (eventos de participação), `matchmaking/006` (invasão), `matchmaking/007` (co-op por sinais), `achievements/009` + `analytics/004` (sinks). Roadmap: [`host_world_dropin-roadmap.md`](./host_world_dropin-roadmap.md). Follow-ups: [`host_world_dropin-follow-ups.md`](./host_world_dropin-follow-ups.md).

## O arquétipo em uma frase

Um **host** abre um **mundo persistente** (sessão sem match prévio); phantoms entram por **papel** (`coop`/`invader`) ocupando **slots por papel**; o server do mundo reporta **eventos por participante** ao longo da vida da sessão (não um `win/loss/draw` no fim).

## Como isto difere do `queue_arena` (fronteira do contrato)

| Eixo | `queue_arena` | `host_world_dropin` |
|---|---|---|
| Origem da sessão | um **match** formado por fila (`CreateSession`) | um **host** abre o mundo (`OpenWorld`), sem match |
| Identidade da sessão | `match_id` | `session_id` (o mundo) |
| Composição | times simétricos (`team_id`) | **slots por papel** (`role` = host/coop/invader), capacidade dinâmica |
| Entrada | todos juntos no início | **join/leave no meio** (`JoinWorld`/`LeaveWorld`) por papel |
| Resultado | **1** `ReportResult(win_loss_draw)` terminal | **N** `ReportParticipationEvent` por participante ao longo da vida |
| Claim no token | `team_id` | **`role`** |
| Envelope de fan-out | `kind=win_loss_draw` | `kind=participation_event` (mesmo tópico `match-results`) |

**Princípio de fronteira (ADR-0010):** o data plane roda o gameplay e **sabe o que aconteceu** (quem matou o host, quem ajudou no boss, quem saiu). O control plane é dono do **ciclo de vida do mundo e dos slots**; o server do mundo **reporta eventos** e o control plane **aplica o efeito no slot** (libera/encerra) e faz o fan-out. O server **não** decide "quem venceu" globalmente — não há vitória global; cada sink interpreta o desfecho por papel.

## Dois fluxos de entrada (o data plane recebe injeções; não forma partida)

Diferente do `queue_arena` (o matchmaking **cria** a sessão), aqui o matchmaking **descobre um mundo existente e injeta** um participante nele. Há dois fluxos assimétricos:

### A) Invasão — busca + injeção automática (o host não escolhe)

```mermaid
sequenceDiagram
  participant Host as Host (Client Unity)
  participant SS as session
  participant World as Server do Mundo (Unity)
  participant Inv as Invasor (Client Unity)
  participant MM as matchmaking

  Host->>SS: OpenWorld(host_player_id, game_id, soul_level, weapon_tier) [JWT]
  SS->>SS: lê perfil, aloca server, deriva slots por papel
  SS-->>Host: session_id + server_address + host_grant(connect_token, role=host)
  Host->>World: conecta com o token (role=host)
  Inv->>MM: Invade(game_id, soul_level, weapon_tier) [JWT]
  MM->>SS: ListInvadableWorlds(game_id)
  SS-->>MM: worlds[]{session_id, host_soul_level, weapon_tier, free_invader_slots}
  MM->>MM: filtra por faixa assimétrica (Eligible)
  MM->>SS: JoinWorld(session_id, invasor, role=invader)
  SS-->>MM: connect_token(role=invader) + server_address
  MM-->>Inv: session_id + server_address + connect_token
  Inv->>World: conecta com o token (role=invader)
  Note over World: mundo autoritativo; phantom injetado no meio
```

### B) Co-op — quadro de sinais em dois passos (o host escolhe)

```mermaid
sequenceDiagram
  participant Ally as Aliado (Client Unity)
  participant MM as matchmaking
  participant Host as Host (Client Unity)
  participant SS as session
  participant World as Server do Mundo (Unity)

  Ally->>MM: PostSummonSign(game_id, soul_level, weapon_tier) [JWT] (stream)
  MM-->>Ally: POSTED{sign_id}  (aguarda invocação)
  Host->>MM: ListSigns(game_id, host_soul_level, host_weapon_tier) [JWT]
  MM-->>Host: signs[]{sign_id, player_id, soul_level, weapon_tier}
  Host->>MM: SummonSign(session_id, sign_id) [JWT]
  MM->>SS: JoinWorld(session_id, aliado, role=coop)
  SS-->>MM: connect_token(role=coop) + server_address
  MM-->>Ally: SUMMONED{session_id, server_address, connect_token}  (pelo stream aberto)
  Ally->>World: conecta com o token (role=coop)
```

> **O que o server do mundo precisa saber destes fluxos:** nada sobre invasão/sinais/faixa — isso é control plane. O server só recebe **conexões novas** (via connect token com `role`) a qualquer momento e as trata como **join-in-progress**. A capacidade por papel já foi garantida pelo `JoinWorld` (o control plane recusa com `ErrRoleFull`); o server confia no token.

## Connect token (session → data plane)

JWT **EdDSA** (Ed25519), validado **offline** pelo server do mundo via JWKS (`/.well-known/jwks.json` do session). Mesmos claims do `queue_arena`, com **`role`** no lugar de `team_id`:

| Claim | Descrição | Nota |
|---|---|---|
| `sub` | playerID | — |
| `match_id` | **o `session_id` do mundo** (o refID do token) | ⚠️ o claim se chama `match_id` por herança, mas **carrega o `session_id`** neste arquétipo |
| `server` | endereço do server do mundo | — |
| `iat` / `exp` | emissão / expiração (TTL 5 min) | — |
| **`role`** | **`"host"` \| `"coop"` \| `"invader"`** | **host_world_dropin** (spec `011`); `team_id` **ausente** aqui |

O server do mundo lê o **`role`** no *connection approval* do NGO (mesmo ponto onde valida o token) para decidir spawn, hostilidade (co-op sem friendly-fire; invasor PvP), HUD e regras por papel. O `match_id` do claim é o `session_id` que o server usa como identidade do mundo ao reportar eventos.

## ReportParticipationEvent (data plane → session)

O **server do mundo** é quem chama — ao longo da vida da sessão, um por evento. RPC unary novo (spec `013`):

```
session.v1.Session/ReportParticipationEvent(
  session_id, player_id, role, event, outcome
) -> {}
```

- **`session_id`**: o mundo (o `match_id` do connect token do participante).
- **`player_id`** / **`role`**: quem e em que papel (o server lê o `role` do token de cada participante).
- **`event`** ∈ `{ JOINED, BOSS_DEFEATED, HOST_KILLED, DIED, LEFT }`.
- **`outcome`** ∈ `{ SUCCESS, FAIL, NEUTRAL }` — interpretado **pelo papel** no sink (não pelo session).

Erros: `InvalidArgument` (event/outcome fora do vocabulário), `NotFound` (mundo inexistente/já encerrado — sinal para o server **parar** de reenviar aquele evento).

### Efeito de cada evento no mundo (o control plane aplica)

| `event` | Quem reporta (típico) | Efeito no mundo (session) |
|---|---|---|
| `JOINED` | o próprio phantom ao entrar | nenhum (o slot já foi ocupado no `JoinWorld`; é a confirmação) |
| `BOSS_DEFEATED` | co-op (ajudou) | nenhum (progresso; o phantom pode sair depois com `LEFT`) |
| `DIED` | phantom morto (invasor abatido / co-op morto) | **libera o slot** do `player_id` (no-op se já ausente) |
| `LEFT` | phantom que sai voluntariamente | **libera o slot** do `player_id` |
| `HOST_KILLED` | o invasor que matou o host | **encerra o mundo** (`CloseReason=host_killed`), libera o server, encerra a participação de todos |

**Ordem no control plane:** o session **publica** o evento no fan-out **antes** de mutar o slot (ADR-0020) — se a publicação falha, o estado não muda e o server **refaz** o report (retry). Logo, o server deve tratar `ReportParticipationEvent` como **idempotente/retriável**: em erro de rede, reenviar; ao receber `NotFound`, parar (o mundo já fechou).

> **Semântica de papel:** o session **não** valida se o `role` combina com o `event` (ex.: "invasor não pode reportar BOSS_DEFEATED") — o server do mundo é **autoritativo**. Ver follow-up #12 se essa validação virar necessária.

## Envelope de fan-out (`kind=participation_event`)

O session publica cada evento no **mesmo** tópico `match-results` do `queue_arena`, discriminado por `kind` (decisão #3 do roadmap — retrocompatível; consumers que não entendem ignoram). O data plane **não** consome isto (é interno ao control plane), mas está aqui como fonte de verdade do que os sinks veem:

```json
{
  "schemaVersion": 2,
  "kind": "participation_event",
  "gameId": "<jogo>",
  "sessionId": "<mundo>",
  "playerId": "<participante>",
  "role": "host | coop | invader",
  "event": "JOINED | BOSS_DEFEATED | HOST_KILLED | DIED | LEFT",
  "outcome": "SUCCESS | FAIL | NEUTRAL"
}
```

Sinks (control plane, já implementados): **achievements** reage por papel (`invader`+`HOST_KILLED`+`SUCCESS` → `invasion_won`; `coop`+`BOSS_DEFEATED`+`SUCCESS` → `coop_boss_helped`; `JOINED` → `world_joined`), idempotente por `(sessionId, playerId, event)`. **analytics** agrega `(gameId, role, event)`.

## Ciclo de vida do mundo e dos slots (referência)

```mermaid
stateDiagram-v2
  [*] --> Aberto: OpenWorld (host, role=host)
  Aberto --> Aberto: JoinWorld (coop/invader) ocupa slot do papel
  Aberto --> Aberto: DIED/LEFT libera slot · BOSS_DEFEATED/JOINED (sem efeito de slot)
  Aberto --> Fechado: HOST_KILLED (host_killed)
  Aberto --> Fechado: host LeaveWorld/CloseWorld (host_left)
  Aberto --> Fechado: TTL sweeper (ttl) · reconciliação (abandoned)
  Fechado --> [*]
```

- **Capacidade por papel** vem do perfil (`EnabledRoles`, `CoopSlots`, `InvaderSlots`). O host **não** ocupa slot (é o dono).
- **Host sai/morre encerra tudo:** `HOST_KILLED` (evento do server) ou `LeaveWorld(host)`/`CloseWorld` (voluntário) fecham o mundo; os phantoms voltam aos seus mundos.

## Mudanças necessárias no `go-game-host-world-dropin-dataplane` (a construir)

O repo Unity ainda não existe. Ao criá-lo, o contrato acima define o que ele precisa implementar. Trabalho cross-repo (não bloqueia o control plane):

| # | Mudança | Origem | Prioridade |
|---|---|---|---|
| 1 | Gerar os stubs C# dos protos `session.v1` (com `ReportParticipationEvent`) e `matchmaking.v1` (Invade/PostSummonSign/ListSigns/SummonSign). | specs `011–013`, `006–007` | **Alta** |
| 2 | Ler o claim **`role`** do connect token no *connection approval* e aplicar no gameplay (spawn/hostilidade/HUD por papel: host, co-op sem friendly-fire, invasor PvP). | spec `011` | **Alta** |
| 3 | Aceitar **join-in-progress** a qualquer momento (phantoms entram no meio) e **leave** (phantom some, mundo segue). Sem reconnect do phantom. | ficha 01 §ciclo de vida | **Alta** |
| 4 | Reportar **eventos por participante** via `ReportParticipationEvent` (`JOINED` ao entrar; `BOSS_DEFEATED`/`HOST_KILLED`/`DIED`/`LEFT` conforme o gameplay), com `outcome` por papel. **Idempotente/retriável** (reenviar em erro; parar em `NotFound`). | spec `013` | **Alta** |
| 5 | Usar o `session_id` (claim `match_id` do token) como identidade do mundo ao reportar. | spec `013` | Alta |
| 6 | (Evolução) `eventId` único por evento para contagem fiel de ocorrências múltiplas na mesma sessão. | follow-up #4 | Média |
| 7 | (Evolução) preencher `stats` nos eventos (se conquistas por stat entrarem). | follow-up #6 | Baixa |

> **Nota de topologia.** Este arquétipo tem afinidade com **P2P** (host como listen-server), mas o MVP usa **dedicated** (Agones/Unity) pela autoridade neutra (ADR-0008). O contrato acima vale para dedicated; a variante `p2p` é um investimento à parte (follow-up/aberturas do roadmap) — muda só a camada de provisionamento/connect da sessão, não os eventos nem os papéis.

## Resumo por papel (o que muda no gameplay)

| Papel | Como entra | Claim `role` | Hostilidade | Eventos típicos que reporta |
|---|---|---|---|---|
| `host` | `OpenWorld` (abre o mundo) | `host` | dono do mundo | — (é alvo; sua morte = `HOST_KILLED`) |
| `coop` | `SummonSign` (host invoca um sinal) | `coop` | aliado, **sem** friendly-fire | `JOINED`, `BOSS_DEFEATED`, `LEFT`, `DIED` |
| `invader` | `Invade` (injeção automática por faixa) | `invader` | PvP contra o host | `JOINED`, `HOST_KILLED` (SUCCESS), `DIED` (FAIL), `LEFT` |
