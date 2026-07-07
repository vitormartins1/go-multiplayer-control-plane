# Plan: 007-battle-royale-profile

**Serviço:** game-registry · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 · **Roadmap:** BR0

## Objetivo

Adicionar o **terceiro arquétipo** (`battle_royale`) ao perfil de multiplayer do game-registry, reusando exatamente o mecanismo polimórfico já em produção (specs `006` para `queue_arena` e o `host_world_dropin` adicionado depois). Mudança **puramente aditiva**: um novo ponteiro no domínio + um novo `case` em cada switch por arquétipo (service, repository, transport). **Sem migration** (coluna JSONB `multiplayer_profile` já existe). Sem tocar matchmaking/session (fatias BR1–BR3).

## Decisões técnicas

### D1 — Reusar o padrão "tagged union por ponteiro" (GO-LEARNINGS, já estabelecido)

- `MultiplayerProfile` ganha `BattleRoyale *BattleRoyaleParams`, não-nil sse `Archetype == "battle_royale"`. Espelha `QueueArena`/`HostWorldDropIn`. Nenhuma decisão nova de modelagem — é aplicar o padrão existente.

### D2 — Sem migration: a coluna JSONB já acomoda o novo `params`

- O `battle_royale` só adiciona um shape de `params` dentro do envelope `{archetype, topology, params}` existente. O `decodeProfile` de perfis antigos (`queue_arena`/`host_world_dropin`) é intocado; um `battle_royale` só é lido quando `sp.Archetype == "battle_royale"`. Zero backfill, zero `ALTER TABLE`.

### D3 — Validação: `lobbySize ≥ 2 × teamSize` além de `teamSize ≥ 1`

- `validateBattleRoyale` (novo, no `profile_service.go`, no estilo de `validateQueueArena`/`validateHostWorldDropIn`):
  - `teamSize ≥ 1` (senão `ErrInvalidProfileParams`).
  - `lobbySize ≥ 2` **e** `lobbySize ≥ 2 * teamSize` (o BR precisa de ao menos dois lados para "último de pé").
  - `respawnMode ∈ {one_life, teammate_revive, arena_1v1, reboot_timer}` (map de vocabulário `brRespawnModes`, no estilo de `qaWinConditions`).
  - cada `scoringStats` casa `scoringStatRE` (reusa a regex já existente de pacote — **não** duplicar).
- **Sem** exigir `lobbySize` múltiplo de `teamSize` (squads incompletos são completados na BR1).

### D4 — Ordem de validação inalterada

- `SetMultiplayerProfile` já valida `archetype` (via `supportedArchetypes`) → `topology` → dispatch no `switch p.Archetype`. Só adiciono `domain.ArchetypeBattleRoyale: true` ao mapa e o `case domain.ArchetypeBattleRoyale` (checa ponteiro não-nil + `validateBattleRoyale`). `ErrUnsupportedArchetype`/`ErrUnsupportedTopology`/`ErrInvalidProfileParams` já existem — nenhum erro novo.

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `internal/domain/game.go` | const `ArchetypeBattleRoyale`; consts de respawn (`RespawnOneLife`/`…`); tipo `BattleRoyaleParams{ LobbySize, TeamSize int; RespawnMode string; ScoringStats []string }`; ponteiro `BattleRoyale *BattleRoyaleParams` em `MultiplayerProfile` (+ atualizar o comentário do invariante) |
| `internal/service/profile_service.go` | `supportedArchetypes` += `battle_royale`; `case domain.ArchetypeBattleRoyale` no `switch`; `validateBattleRoyale`; map `brRespawnModes` |
| `internal/repository/postgres_profile.go` | `storedBattleRoyale`; `case domain.ArchetypeBattleRoyale` em `encodeProfile` e `decodeProfile` |
| `internal/transport/http_handler.go` | `battleRoyaleParamsDTO`; `case domain.ArchetypeBattleRoyale` em `profileToDomain` e `profileToDTO` |
| `cmd/server/main.go` | **sem mudança** |
| `migrations/` | **sem mudança** (D2) |

**Nenhum erro de domínio novo, nenhuma rota nova** — o `battle_royale` só amplia os valores aceitos nos endpoints da spec 006.

## Testes (mapeiam os casos da spec)

- **service** (`profile_service_test.go`, mock do repo): upsert `battle_royale` válido; `teamSize<1`; `lobbySize<2` e `lobbySize<2×teamSize` (ex.: `6/4`); cada `respawnMode` válido aceito + um inválido; `scoringStats` bom/ruim; `ErrGameNotFound` propagado; **regressão**: um `queue_arena` e um `host_world_dropin` válidos seguem aceitos. Cobertura ≥ 80%.
- **repo** (`postgres_game_test.go`, testcontainers): upsert/`FindByID` round-trip do `battle_royale`; **reconfiguração entre arquétipos** (ex.: jogo com `queue_arena` → `battle_royale`) substitui e limpa o ponteiro antigo.
- **transport** (`http_handler_test.go`, `httptest`): `PUT` com `battle_royale` → 200/400; `GET /games/{id}` inclui o `multiplayerProfile` `battle_royale` (round-trip DTO↔domínio).

## Observabilidade

Sem trabalho novo: reusa as rotas da spec 006 (já em métricas RED + span por rota). Mesmo banco → sem exporter novo.

## Riscos / fora do escopo

- **Risco baixo:** aditivo, sem migration, sem contrato alterado. O único ponto de atenção é **não esquecer** um dos 4 switches (service/repo encode/repo decode/transport) — as tasks cobrem os quatro e o teste de round-trip repo+transport pega omissão.
- Consumo do perfil (matchmaking BR1, session BR2, sinks BR3) e o **form no admin-ui** (spec-companheira) ficam fora daqui.
