# Mapa de features — backlog por serviço

> **Panorama e estado:** [`ARCHITECTURE.md`](ARCHITECTURE.md) (mapa de serviços) · [`README.md`](../README.md) de cada serviço · [`NEXT-STEPS.md`](NEXT-STEPS.md) (backlog priorizado do momento). Este doc foi enxugado (2026-07-07): as features `[MVP]`/`[próximo]` já foram construídas e os "concerns compartilhados" já foram decididos (auth via `shared/auth`, connect token Ed25519, fan-out SNS→SQS ADR-0020, contratos `.proto`). Ficou o que ainda **não** foi feito.

O loop MVP (`login → find match → aloca server + connect token → partida → resultado → leaderboard/achievements/analytics/persistência`) está **fechado e observável** no painel. Cada linha abaixo é candidata a uma feature SDD futura (`NNN-slug`), a ser escrita just-in-time.

## Backlog de features por serviço (o que falta)

| Serviço | Features futuras (não construídas) |
|---|---|
| identity | login federado (Steam/Google); account linking (guest → conta) |
| achievements | conquistas secretas / recompensas associadas |
| player-data | controle de concorrência (optimistic locking); quota por jogador; bulk get/set |
| cloud-save | múltiplos slots; compressão / sync delta |
| leaderboard | múltiplos boards por modo/temporada; **ranking de invasões/co-ops** (host_world_dropin, cauda); anti-cheat de score; seasons nomeadas (adiado, precisa de ADR sobre onde vive a config) |
| analytics | telemetria genérica (`POST /events`); sessionization / funis / retenção; série temporal + stats brutos pros arquétipos além de `queue_arena` (Fase 2 do dashboard por jogo, gated por evidência) |
| matchmaking | agrupamento por latência/região; relaxamento progressivo; party/grupos, backfill; **pools por região** (host_world_dropin, follow-up) |
| session | alocação de server **sob demanda** (host_world_dropin, follow-up); reconnection / spectator / migração de sessão |
| game-registry | purge assíncrono do soft delete (ADR-0025, Opção C — gated por evidência) |

## Backlog por arquétipo (ADR-0023)

Features de multiplayer que ainda não têm arquétipo implementado, ou evoluções dos que têm:

- **Os 4 arquétipos estão completos no control plane** (`queue_arena`, `host_world_dropin`, `battle_royale`, `coop_mission`) — ver `game-models/README.md` (matriz de cobertura). O que falta é **cross-repo**: só `queue_arena` tem data plane Unity de verdade; os outros 3 têm contrato pronto, sem repo Unity ainda (`UNITY-INTEGRATION.md`).
- **Evoluções do `host_world_dropin`:** consolidadas em [`game-models/host_world_dropin-follow-ups.md`](game-models/host_world_dropin-follow-ups.md) (pools por região, Redis+pub/sub, alocação sob demanda, etc. — senha e critério por stat já foram fechados).
- **Topologia `p2p`:** campo no perfil (ADR-0023); implementação = ADR futuro + data plane próprio.
