# Tasks: 008-coop-mission-profile

**Serviço:** game-registry · **Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Ordem hexagonal: domínio → repositório → serviço → transporte → UI → verificação. Sem migration (D2). Marcar `[x]` conforme conclui.

## Domínio (ADR-0002 — zero deps externas)

- [x] **T1** — Em `internal/domain/game.go`: const `ArchetypeCoopMission = "coop_mission"` (junto das existentes).
- [x] **T2** — Em `game.go`: tipo `CoopMissionParams{ TeamSize int; Missions []string; Difficulties []string; BotfillEnabled bool; ScoringStats []string }` com doc-comment (ficha 04/ADR-0023). Adicionar `CoopMission *CoopMissionParams` ao `MultiplayerProfile` e atualizar o comentário do invariante + o comentário do campo `Archetype`.

## Repositório (pgx) — sem migration

- [x] **T3** — Em `internal/repository/postgres_profile.go`: `storedCoopMission{ TeamSize int json:"teamSize,omitempty"; Missions []string json:"missions,omitempty"; Difficulties []string json:"difficulties,omitempty"; BotfillEnabled bool json:"botfillEnabled"; ScoringStats []string json:"scoringStats,omitempty" }`.
- [x] **T4** — `postgres_profile.go`: `case domain.ArchetypeCoopMission` em `encodeProfile` (nil-check → `perfil sem params`; marshal do `storedCoopMission`) e em `decodeProfile` (unmarshal → `p.CoopMission = &domain.CoopMissionParams{...}`).

## Serviço (regra de negócio)

- [x] **T5** — Em `internal/service/profile_service.go`: `supportedArchetypes` += `domain.ArchetypeCoopMission: true`.
- [x] **T6** — `profile_service.go`: `validateCoopMission(p domain.CoopMissionParams) error` (D3): `teamSize ≥ 2`; `missions`/`difficulties` cada item casa `scoringStatRE` e sem duplicata (dedupe local); `scoringStats` cada item casa `scoringStatRE` (sem dedupe). Retorna `ErrInvalidProfileParams`.
- [x] **T7** — `profile_service.go`: `case domain.ArchetypeCoopMission` no `switch p.Archetype` de `SetMultiplayerProfile` — checa `p.CoopMission != nil` (senão `ErrInvalidProfileParams`) + `validateCoopMission`. Ordem geral inalterada.

## Transporte (HTTP)

- [x] **T8** — Em `internal/transport/http_handler.go`: `coopMissionParamsDTO{ TeamSize int; Missions, Difficulties []string; BotfillEnabled bool; ScoringStats []string }` (JSON camelCase).
- [x] **T9** — `http_handler.go`: `case domain.ArchetypeCoopMission` em `profileToDomain` (unmarshal → `domain.CoopMissionParams`) e em `profileToDTO` (marshal do DTO → raw). Espelha os cases existentes.

## Testes (mapeiam os "Casos de Teste Obrigatórios" da spec)

- [x] **T10** — `internal/service/profile_service_test.go`: `coop_mission` válido → upsert; `teamSize<2` → inválido; `missions` item ruim/duplicado → inválido, vazio → ok; `difficulties` ruim/duplicado → inválido, vazio → ok; `scoringStats` ruim → inválido, vazio → ok; `ErrGameNotFound` propagado; **regressão**: `queue_arena`/`host_world_dropin`/`battle_royale` seguem OK. `TestSetMultiplayerProfile_CoopMission_*`. `-race -cover` (≥80%).
- [x] **T11** — `internal/repository/postgres_game_test.go` (testcontainers): upsert/`FindByID` round-trip do `coop_mission`; reconfigurar de `battle_royale` → `coop_mission` substitui e o ponteiro antigo fica nil.
- [x] **T12** — `internal/transport/http_handler_test.go` (`httptest`): `PUT .../multiplayer-profile` com `coop_mission` → 200 e 400; `GET /games/{id}` inclui o `multiplayerProfile` `coop_mission` (round-trip DTO↔domínio).

## UI (companheira C0-UI)

- [x] **T13** — `services/admin-ui`: union `coop_mission` em `src/lib/api.ts`; ramo de form condicional em `src/pages/MultiplayerSettings.tsx` (teamSize/missions/difficulties/botfillEnabled/scoringStats, espelha o do `battle_royale`). Verificação: `cd services/admin-ui && npm run build`.

## Verificação final

- [x] **T14 (automático)** — `go build ./...` ✅; unit 119 verdes (`-race`), service cover 89.4% ✅; integração testcontainers 20 verdes ✅; `golangci-lint run ./services/game-registry/...` 0 issues ✅; `gofmt` limpo nos arquivos tocados ✅; `admin-ui` `npm run build` verde ✅.
- [ ] **T15 (smoke manual — opcional, do Vitor)** — docker-compose + collection `.http`: `PUT` perfil `coop_mission` → `GET /games/{id}` mostra → outros arquétipos seguem OK.
- [x] **T16** — Loop de aprendizado (CLAUDE.md): **nada novo de Go** esperado (aplicação do tagged-union por ponteiro já documentado). Confirmar e não inventar.
