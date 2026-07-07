# Plan: 008-coop-mission-profile

**Serviço:** game-registry · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 · **Roadmap:** C0

## Objetivo

Adicionar o **quarto arquétipo** (`coop_mission`) ao perfil de multiplayer do game-registry, reusando exatamente o mecanismo polimórfico já em produção (specs `006`/`007`). Mudança **puramente aditiva**: um novo ponteiro no domínio + um novo `case` em cada switch por arquétipo (service, repository, transport). **Sem migration** (coluna JSONB `multiplayer_profile` já existe). Sem tocar matchmaking/session (fatias C1–C3).

## Decisões técnicas

### D1 — Reusar o padrão "tagged union por ponteiro" (GO-LEARNINGS, já estabelecido)

- `MultiplayerProfile` ganha `CoopMission *CoopMissionParams`, não-nil sse `Archetype == "coop_mission"`. Espelha `QueueArena`/`HostWorldDropIn`/`BattleRoyale`. Nenhuma decisão nova de modelagem — é aplicar o padrão existente.

### D2 — Sem migration: a coluna JSONB já acomoda o novo `params`

- O `coop_mission` só adiciona um shape de `params` dentro do envelope `{archetype, topology, params}`. Perfis antigos são intocados; um `coop_mission` só é lido quando `sp.Archetype == "coop_mission"`. Zero backfill, zero `ALTER TABLE`.

### D3 — Validação: `teamSize ≥ 2` + catálogos como identificadores estáveis sem duplicata

- `validateCoopMission` (novo, no `profile_service.go`, no estilo de `validateBattleRoyale`):
  - `teamSize ≥ 2` (co-op precisa de ao menos dois — ficha usa ~4).
  - `missions`/`difficulties`: cada item casa `scoringStatRE` (reusa a regex de pacote) e **não repete** (dedupe via `map[string]bool` local, no estilo do `EnabledRoles` do `host_world_dropin`). Listas vazias são válidas (o data plane pode ter catálogo próprio).
  - `scoringStats`: cada item casa `scoringStatRE`. **Sem** checagem de duplicata — mantém a convenção idêntica de `queue_arena`/`battle_royale` (não mudar o tratamento de `scoringStats` num arquétipo só).
- **Por que dedupe em missions/difficulties mas não em scoringStats:** os primeiros são **catálogos** (repetir é erro de cadastro sem sentido); `scoringStats` segue a convenção pré-existente do serviço (regex-only) para não introduzir inconsistência entre arquétipos.

### D4 — Ordem de validação inalterada

- `SetMultiplayerProfile` já valida `archetype` (via `supportedArchetypes`) → `topology` → dispatch no `switch p.Archetype`. Só adiciono `domain.ArchetypeCoopMission: true` ao mapa e o `case domain.ArchetypeCoopMission` (checa ponteiro não-nil + `validateCoopMission`). `ErrUnsupportedArchetype`/`ErrUnsupportedTopology`/`ErrInvalidProfileParams` já existem — nenhum erro novo.

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `internal/domain/game.go` | const `ArchetypeCoopMission`; tipo `CoopMissionParams{ TeamSize int; Missions, Difficulties []string; BotfillEnabled bool; ScoringStats []string }`; ponteiro `CoopMission *CoopMissionParams` em `MultiplayerProfile` (+ atualizar comentário do invariante e do campo `Archetype`) |
| `internal/service/profile_service.go` | `supportedArchetypes` += `coop_mission`; `case domain.ArchetypeCoopMission` no `switch`; `validateCoopMission` |
| `internal/repository/postgres_profile.go` | `storedCoopMission`; `case domain.ArchetypeCoopMission` em `encodeProfile` e `decodeProfile` |
| `internal/transport/http_handler.go` | `coopMissionParamsDTO`; `case domain.ArchetypeCoopMission` em `profileToDomain` e `profileToDTO` |
| `services/admin-ui/src/lib/api.ts` + `MultiplayerSettings.tsx` | union `coop_mission` + ramo de form condicional (companheira C0-UI) |
| `cmd/server/main.go` · `migrations/` | **sem mudança** (D2) |

**Nenhum erro de domínio novo, nenhuma rota nova** — o `coop_mission` só amplia os valores aceitos nos endpoints da spec 006.

## Testes (mapeiam os casos da spec)

- **service** (`profile_service_test.go`, mock do repo): upsert `coop_mission` válido; `teamSize<2`; `missions` bom/ruim/duplicado; `difficulties` bom/ruim/duplicado; `scoringStats` bom/ruim; `ErrGameNotFound` propagado; **regressão**: um `queue_arena`, um `host_world_dropin` e um `battle_royale` válidos seguem aceitos. Cobertura ≥ 80%.
- **repo** (`postgres_game_test.go`, testcontainers): upsert/`FindByID` round-trip do `coop_mission`; **reconfiguração entre arquétipos** (ex.: jogo com `battle_royale` → `coop_mission`) substitui e limpa o ponteiro antigo.
- **transport** (`http_handler_test.go`, `httptest`): `PUT` com `coop_mission` → 200/400; `GET /games/{id}` inclui o `multiplayerProfile` `coop_mission` (round-trip DTO↔domínio).
- **admin-ui**: `npm run build` (tsc+vite) verde com a variante nova.

## Observabilidade

Sem trabalho novo: reusa as rotas da spec 006 (já em métricas RED + span por rota). Mesmo banco → sem exporter novo.

## Riscos / fora do escopo

- **Risco baixo:** aditivo, sem migration, sem contrato alterado. Ponto de atenção: **não esquecer** um dos 4 switches (service/repo encode/repo decode/transport) — as tasks cobrem os quatro e o round-trip repo+transport pega omissão.
- Consumo do perfil (matchmaking C1, session C2, sinks C3) fica fora daqui.
