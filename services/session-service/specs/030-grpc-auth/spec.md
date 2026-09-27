# Spec: 030-grpc-auth

**Serviço:** session-service (+ matchmaking-service, proto `session.v1`, infra)
**Status:** Done (fatias 1 e 2; virar o padrão para `enforce` depende do jogo — ver "Rollout")
**Data:** 2026-09-26
**ADR:** [0030](../../../../docs/adr/0030-session-grpc-auth.md) · decisão D1 da revisão 2026-09

## Contexto

O gRPC do session não autentica ninguém: abre mundo em nome de qualquer `host_player_id`, aceita resultado e
evento de qualquer um. Detalhe e alternativas na ADR-0030.

## Comportamento

### Credenciais (metadata gRPC)

| Chamador | Metadata | Validação |
|---|---|---|
| Jogador (cliente do jogo) | `authorization: Bearer <JWT do identity>` | `shared/auth.Verifier` (JWKS do identity, `iss`/`aud`) → `player_id` |
| Matchmaking | `x-internal-key: <INTERNAL_API_KEY>` | igual à chave do deploy (ADR-0018) |
| Dedicated Server | `x-game-server-key: <GAME_SERVER_KEY>` | igual à chave do deploy |

### Quem pode chamar cada RPC

| RPC | Aceita |
|---|---|
| `CreateSession`, `ListInvadableWorlds`, `JoinWorld` | matchmaking |
| `OpenWorld` | jogador — o host é o `sub` do token; `host_player_id`, se vier, precisa ser igual |
| `LeaveWorld` | jogador saindo (`player_id` = `sub`) ou o host do mundo · Dedicated Server |
| `CloseWorld` | jogador host do mundo · Dedicated Server |
| `ReportResult`, `ReportParticipationEvent`, `GetSessionConfig` | Dedicated Server |
| reflection / health | aberto |

### Modo (`SESSION_AUTH_MODE`)

- `enforce`: credencial ausente/inválida → `Unauthenticated`; credencial válida sem permissão para o RPC ou para o
  mundo (ex.: não é o host) → `PermissionDenied`.
- `warn` (padrão): a mesma avaliação acontece, mas a falha só é logada (`slog` WARN) e contada em
  `session_auth_rejections_total{method,reason}`; a chamada segue com a semântica anterior (no `OpenWorld`, o host
  é o `sub` do token quando ele é válido; senão, o `host_player_id` do request).
- `off`: sem avaliação (só para testes locais pontuais).

### Invocação de sinal só pelo host (fatia 1)

- `JoinWorldRequest` ganha `summoner_player_id` (campo 4, opcional). WHEN preenchido -> THE SYSTEM SHALL exigir
  que seja o host do mundo, senão `PermissionDenied` (independe do modo: quem manda o campo é o matchmaking, que já
  autentica o jogador).
- O matchmaking preenche o campo no `SummonSign` com o `sub` do JWT de quem invocou.

## Critérios de aceite

1. Interceptor: tabela de permissões coberta por teste (cada tipo de chamador × RPC), nos três modos.
2. `OpenWorld` sem `host_player_id` com bearer válido abre o mundo para o `sub`.
3. `CloseWorld` de jogador que não é o host → `PermissionDenied` (enforce); do host → ok.
4. `SummonSign` num mundo de outro host → `PermissionDenied` no matchmaking.
5. Matchmaking manda `x-internal-key` em toda chamada ao session.
6. Compose sobe em `warn` com `GAME_SERVER_KEY` de dev; fleets recebem a chave por `Secret` opcional.
7. `scripts/demo-host-world.sh` passa com o session em `enforce` (mandando as credenciais).
8. Contrato (`host_world_dropin`, `queue_arena`) e `docs/contracts/CHANGELOG.md` (1.2.0) atualizados.

## Rollout

`warn` até o jogo mandar o bearer no `OpenWorld`/`LeaveWorld`/`CloseWorld` e a chave de servidor nos reports; então
`SESSION_AUTH_MODE=enforce` (compose e fleets). O painel `session_auth_rejections_total` mostra quando zerou.
