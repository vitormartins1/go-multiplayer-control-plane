# session-service

≈ **GameLift (orquestração de frota/sessão)** — **capstone** do control plane (ADR-0010). Não simula nada: a partir de um match ou da abertura de um mundo, aloca um game server, emite o **connect token (Ed25519)** que o Unity Dedicated Server valida offline, e recebe de volta o desfecho (resultado terminal ou eventos por participante), com fan-out para os serviços de dados.

- **Protocolo:** gRPC (unary no MVP) + HTTP (JWKS do connect token, métricas, admin)
- **Storage:** memória (sessões/mundos ativos) + Postgres próprio (`session-postgres`) pro histórico de sessões fechadas — sobrevive a restart, ao contrário do ring in-memory (spec 025)
- **Dependências:** matchmaking (recebe match / injeta phantom), identity (control plane interno); cluster Agones no modo `agones`
- **Multi-tenant por jogo** (ADR-0017)

Suporta **4 arquétipos** por perfil do jogo (game-registry): `queue_arena` e `battle_royale`/`coop_mission` (sessão nasce de um **match**, ciclo de vida terminal — abre, joga, reporta resultado, fecha) e **`host_world_dropin`** (ADR-0023) — mundo persistente que nasce do **client do host**, sem match: phantoms co-op/invasor entram e saem no meio da sessão (join-in-progress) e o servidor reporta **eventos por participante**, não um placar final.

## API — Sessões de partida (queue_arena / battle_royale / coop_mission)

```
service Session {
  rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse);
  rpc ReportResult(ReportResultRequest) returns (ReportResultResponse);
}
```

- **`CreateSession`** — aloca um game server livre, emite um `connect_token` por jogador (`ConnectGrant`) e registra a sessão. Recebe `match_id`, `players`, `game_id`, `teams` (NvN por time; vazio em FFA) e `mode_id`. `ResourceExhausted` se o pool/Fleet esgotar.
- **`ReportResult`** — recebe o desfecho da partida e dispara o fan-out (leaderboard/achievements), libera o server e encerra a sessão. `NotFound` se não houver sessão para o `match_id`. O outcome é **polimórfico** (oneof, ADR-0023) por arquétipo:
  - `WinLossDrawResult` (queue_arena) — win/loss/draw por jogador (+ time via `team_id`) com `stats`.
  - `PlacementResult` (battle_royale) — colocação ordinal (1º..Nº) por participante/squad, com `survived_ms`.
  - `CollectiveResult` (coop_mission) — desfecho **coletivo** da missão (`MISSION_SUCCESS`/`MISSION_FAILURE`), igual para todos; stats individuais só modulam recompensa.

## API — Mundos persistentes (host_world_dropin)

```
service Session {
  rpc OpenWorld(OpenWorldRequest) returns (OpenWorldResponse);
  rpc JoinWorld(JoinWorldRequest) returns (JoinWorldResponse);
  rpc LeaveWorld(LeaveWorldRequest) returns (LeaveWorldResponse);
  rpc CloseWorld(CloseWorldRequest) returns (CloseWorldResponse);
  rpc ListInvadableWorlds(ListInvadableWorldsRequest) returns (ListInvadableWorldsResponse);
  rpc ReportParticipationEvent(ReportParticipationEventRequest) returns (ReportParticipationEventResponse);
  rpc GetSessionConfig(GetSessionConfigRequest) returns (GetSessionConfigResponse);
}
```

Aqui não há match: a identidade do mundo é o próprio `session_id`, derivado na abertura pelo host.

- **`OpenWorld`** (spec 011) — o host abre o mundo: aloca um server, deriva os slots por papel do perfil do jogo e emite o `host_grant` (connect token, claim `role=host`). Recebe `host_player_id`, `game_id`, `host_soul_level`/`weapon_tier` (faixa para matchmaking), `mode_id` e `password` (opcional, spec 020). `FailedPrecondition` se o jogo não for `host_world_dropin`; `ResourceExhausted` se o pool/Fleet esgotar.
- **`JoinWorld`** — injeta um phantom (`role` = `coop` | `invader`) no mundo, ocupa um slot e devolve `{ grant, server_address }`. `InvalidArgument` (papel não habilitado no perfil), `ResourceExhausted` (slots cheios), `NotFound` (mundo inexistente), `FailedPrecondition` (jogador já presente).
- **`LeaveWorld`** — libera o slot do jogador; o mundo **segue aberto**. No-op idempotente se o jogador já não estiver presente. Se quem sai é o host, encerra o mundo.
- **`CloseWorld`** — encerra o mundo (host saiu) e libera o server. `NotFound` se não houver mundo aberto para o `session_id`.
- **`ListInvadableWorlds`** (spec 012) — devolve os mundos abertos do jogo com vaga de invasor livre + atributos do host (`host_soul_level`, `weapon_tier`, `free_invader_slots`, `password_hash`). Fonte de dados pura para o matchmaking casar por faixa/senha — sem filtro aqui; lista vazia se nada casar.
- **`ReportParticipationEvent`** (spec 013) — o server do mundo reporta um evento por participante ao longo de toda a vida da sessão: `role` (`host`|`coop`|`invader`) × `event` (`JOINED`|`BOSS_DEFEATED`|`HOST_KILLED`|`DIED`|`LEFT`) × `outcome` (`SUCCESS`|`FAIL`|`NEUTRAL`), com `event_id` opcional (spec 021, dedup em sinks) e `stats` opcional (`map<string,int64>`, spec 026 — kills/revives/etc., mesmo padrão de `ReportResult`; pendente confirmar se o Dedicated Server Unity já preenche). Publica no fan-out (`kind=participation_event`) e aplica o efeito no slot: `DIED`/`LEFT` liberam; `HOST_KILLED` encerra o mundo. `InvalidArgument` (vocabulário inválido), `NotFound` (mundo inexistente/fechado). Idempotente/retriável — o publish ocorre antes da mutação do slot.
- **`GetSessionConfig`** (ADR-0024, spec 017) — devolve o **session manifest**: o snapshot do perfil do modo congelado na criação (`archetype`, `mode_id`, `params_json`, `profile_hash`). Chamado pelo game server no boot da sessão; `ref_id` aceita `match_id` (arquétipos de partida) ou `session_id` (host_world). `NotFound` se a sessão não existir ou já estiver fechada. O data plane lê só as chaves de `params_json` que conhece — nenhuma capacidade/regra fica hardcoded no client.

## Admin (HTTP)

Rotas sem auth no serviço — internal key na borda (ADR-0018), mesmo padrão dos pares:

| Método | Rota | Descrição |
|---|---|---|
| GET | `/admin/games/{gameId}/sessions` | sessões de um jogo — sem `state=`, ativas (mapa) + fechadas recentes (ring in-memory); com `state=closed`, fechadas **persistidas** no Postgres, paginadas por `limit`/`before` (spec 025). `503 history_unavailable` se o Postgres do histórico estiver fora |
| GET | `/admin/players/{playerId}/sessions?gameId=` | sessões fechadas do jogo em que o player participou (aba History, spec 025), mesma paginação por cursor |
| GET | `/admin/sessions` | board global de sessões, todos os jogos (spec 007) |
| POST | `/admin/sessions/{matchId}/close` | force-close administrativo; o path aceita **`matchId` OU `sessionId`** (spec 024) — resolve pelo caminho certo conforme o arquétipo da sessão |
| GET | `/admin/fleet-status/{fleetName}` | status real da Fleet Agones (réplicas, ready/allocated) — só populado no modo `agones` (spec 023) |

## Allocator (spec 004)

- `ALLOCATOR_MODE=static` (default): pool mock em memória (`SERVER_POOL`) — compose e testes funcionam sem cluster.
- `ALLOCATOR_MODE=agones`: cria `GameServerAllocation` via k8s API (dynamic client) com annotations `match_id`/`game_id` (chave **sem prefixo** — validado empiricamente; o Unity lê `objectMeta.annotations` cru e rearma o gate). Envs: `KUBECONFIG` (ou in-cluster), `AGONES_NAMESPACE` (default `default`). A **Fleet não é mais fixa por env var** (ADR-0026, spec 022): cada sessão resolve a Fleet do perfil do jogo (game-registry, `PUT /games/{id}/fleet` — spec `game-registry/012`). **Runbook:** antes de habilitar `ALLOCATOR_MODE=agones` para um jogo, (1) crie a Fleet correspondente no cluster (k3d/Pi5) e (2) cadastre o nome no jogo via `PUT /games/{id}/fleet`; sem isso, a 1ª `CreateSession`/`OpenWorld` do jogo falha com `FailedPrecondition` ("fleet not configured") em vez de arriscar alocar do binário errado.
- `DEV_PUBLIC_HOST` (dev): reescreve o host do endereço devolvido mantendo a porta dinâmica (IP interno do node k3d → `127.0.0.1`; portas mapeadas pelo k3d).
- Sessões de partida fecham por `ReportResult` (reason `result`) ou pelo sweeper de TTL (`SESSION_TTL`, default `30m`, reason `ttl`); mundos fecham por `CloseWorld`/host saindo (reason `host_left`) ou TTL. Fechar = `Release` (no Agones, deleta o GameServer — a Fleet repõe).
- Compose com k3d: `docker compose -f docker-compose.yml -f docker-compose.agones.yml up -d session-service` (ver `infra/docker/docker-compose.agones.yml`).

## Observabilidade

Métricas: `session_active_sessions` (gauge), `sessions_closed_total{reason}` (counter), `session_history_persist_failures_total` (counter — falha ao persistir o histórico, spec 025; fechamento nunca depende disso). `session_auth_rejections_total{method,reason}` (counter — chamadas gRPC sem credencial válida ou sem permissão, spec 030; em `warn` a chamada segue). `/metrics` na porta do serviço (**Service Overview**, `$job=session-service`). Tracing no Tempo. Status real de Fleet (réplicas, allocated) exposto via `/admin/fleet-status/{fleetName}` no modo `agones` (spec 023).

## Testar / Specs

Uso real em homelab (k3d local + Raspberry Pi 5) via Agones, validado ponta a ponta com o data plane Unity dos dois arquétipos (`unity-queue-arena-data-plane`, `unity-host-world-dropin-data-plane`).

[`specs/`](specs/) — 24 specs, agrupadas por tema:

- **Fundação (partida):** `001-session-core` · `002-result-fanout` · `003-leaderboard-fanout` · `005-admin-sessions` · `006-durable-fanout` · `007-admin-session-control` · `008-orphan-reconcile` · `009-polymorphic-result` · `010-team-registration` · `015-placement-result` · `016-collective-result`
- **host_world_dropin (ADR-0023):** `011-host-world-session` · `012-invadable-world-directory` · `013-participation-events` · `014-host-world-board-fields` · `020-password-world-matching` · `021-participation-event-id` · `024-close-room-host-world-fix`
- **Allocator / Fleet:** `004-agones-allocator` · `022-fleet-aware-allocation` · `023-fleet-status`
- **Manifest / admin console:** `017-mode-snapshot-manifest` · `018-admin-session-detail` · `019-admin-session-archetype-layout`

## Autenticação do gRPC (spec 030, ADR-0030)

| Env | Uso |
|---|---|
| `SESSION_AUTH_MODE` | `warn` (padrão: avalia, loga e conta, não bloqueia) · `enforce` · `off` |
| `IDENTITY_JWKS_URL` | JWKS do identity para validar o bearer do jogador |
| `INTERNAL_API_KEY` | chave do matchmaking (metadata `x-internal-key`) e do `/admin` HTTP |
| `GAME_SERVER_KEY` | chave do Dedicated Server (metadata `x-game-server-key`) |

Quem pode chamar cada RPC: `specs/030-grpc-auth/spec.md`.

## Topologia p2p (spec 031, ADR-0032)

Modo `host_world_dropin` com `topology: p2p`: o `OpenWorld` não aloca GameServer — exige `host_connection` (id de
conexão do host, ex.: SteamID64), devolve um `world_token` e repassa o `host_connection` a quem entra. O jogo do host
reporta com a metadata `x-world-token`, que só vale para o próprio mundo e morre com ele. Sem GameServer, o
reconcile de órfãs ignora esses mundos; o sweeper de TTL fecha o mundo se o host sumir. Demo:
`TOPOLOGY=p2p scripts/demo-host-world.sh`.

