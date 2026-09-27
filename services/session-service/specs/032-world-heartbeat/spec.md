# Spec: 032-world-heartbeat

**Serviço:** session-service (proto `session.v1`)
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** follow-up da spec `031-p2p-topology`

## Contexto

O sweeper fecha qualquer sessão aberta há mais que `SESSION_TTL` (30 min) — idade, não inatividade. Para partida
curta (`queue_arena`) está certo. Para mundo do `host_world_dropin` é errado dos dois lados: um mundo saudável de
Souls-like dura horas e é derrubado aos 30 min; e, em p2p, o TTL é o único sinal de que o host caiu, então o mundo
de um host que crashou fica listado para invasão por até 30 min.

## Comportamento (EARS)

1. O session **deve** oferecer o RPC `Heartbeat(session_id)` para mundos abertos (`host_world`), aceito do jogo do
   host (bearer do host ou `x-world-token` do mundo) e do Dedicated Server (`x-game-server-key`).
2. **Quando** receber um heartbeat, o session **deve** registrar o instante e responder `interval_seconds` — de quanto
   em quanto tempo mandar o próximo (um terço de `WORLD_HEARTBEAT_TTL`).
3. **Enquanto** um mundo já tiver recebido heartbeat, o sweeper **deve** fechá-lo só quando o último heartbeat for
   mais velho que `WORLD_HEARTBEAT_TTL` (padrão 90 s), com motivo `abandoned` — e **não** mais pela idade.
4. **Enquanto** um mundo nunca tiver recebido heartbeat (jogo ainda não atualizado), o sweeper **deve** seguir a regra
   de hoje (idade > `SESSION_TTL`, motivo `ttl`). Compatibilidade: ninguém que não manda heartbeat muda de
   comportamento.
5. **Se** o `session_id` não for de um mundo aberto, o RPC **deve** responder `NotFound` (o jogo para de mandar e,
   se quiser, reabre o mundo).
6. Partidas (`queue_arena` e afins) não mudam.

## Contrato (aditivo — contrato 1.5.0)

- `rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse)`
- `HeartbeatRequest { string session_id = 1; }` · `HeartbeatResponse { int32 interval_seconds = 1; }`

## Critérios de aceite

- Unit: mundo com heartbeat recente sobrevive à idade > `SESSION_TTL`; heartbeat vencido fecha como `abandoned`;
  mundo sem heartbeat segue a regra antiga; `Heartbeat` em mundo inexistente → `ErrWorldNotFound`.
- Transport: permissões (host por bearer, world token só do próprio mundo, DS; matchmaking e outro jogador não).
- `buf breaking` verde; CHANGELOG 1.5.0; contrato do arquétipo atualizado.
- Métrica `session_world_heartbeats_total`.
