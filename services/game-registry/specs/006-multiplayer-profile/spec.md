# Spec: 006-multiplayer-profile

**Serviço:** game-registry
**Status:** Implementada (2026-07-06)
**Data:** 2026-07-06
**ADR:** 0023 (perfil de multiplayer por jogo — arquétipo + parâmetros)

## Contexto

Primeira fatia do ADR-0023: o **game-registry** (fonte canônica de jogos, ADR-0017) passa a guardar, por jogo, um **perfil de multiplayer** — a declaração da *forma* do multiplayer que os demais serviços (matchmaking, session) vão ler para se configurar. Esta feature cobre **só o cadastro** no game-registry: persistir, validar e expor o perfil. **Não** toca matchmaking, session, nem o modelo de resultado (fatias seguintes).

Escopo restrito ao arquétipo **`queue_arena`** (ADR-0023): sessão simétrica de N jogadores, FFA ou por times, que termina com um desfecho e opcionalmente stats. É o arquétipo mais próximo do backend atual (o veículo de arena que já existe) — o menor salto para validar o mecanismo de perfil. Os demais arquétipos (`battle_royale`, `host_world_dropin`, `coop_mission`) ficam **fora do escopo** desta feature.

O perfil é **opcional** no ciclo de vida do jogo: um jogo pode existir sem perfil (registrado por `001-game-core`) e ser configurado depois. É **control plane interno**, atrás da internal key como os demais `/games/...` de escrita (ADR-0018).

## Comportamento

### Definir/atualizar o perfil de multiplayer

- WHEN `PUT /games/{id}/multiplayer-profile` com um corpo de perfil **válido** → THE SYSTEM SHALL fazer **upsert** do perfil do jogo (cria se ausente, substitui se presente) e retornar `200` com o perfil persistido.
- IF o jogo `{id}` não existir → `404` (**ErrGameNotFound**).
- IF o `archetype` não for suportado → `400` (**ErrUnsupportedArchetype**). No MVP, o único suportado é `queue_arena`.
- IF a `topology` não for suportada → `400` (**ErrUnsupportedTopology**). No MVP, a única suportada é `dedicated` (`p2p` é reservado — ADR-0023 — e **rejeitado** por ora).
- IF os `params` do `queue_arena` forem inválidos → `400` (**ErrInvalidProfileParams**), com a razão (ver "Validação do queue_arena").

### Consultar o perfil

- WHEN `GET /games/{id}` → THE SYSTEM SHALL incluir o campo `multiplayerProfile` na resposta: o perfil se configurado, ou `null` se o jogo ainda não tem perfil.
- WHEN `GET /games/{id}/multiplayer-profile` → retorna `200` com o perfil, ou `404` (**ErrProfileNotFound**) se o jogo existe mas não tem perfil; `404` (**ErrGameNotFound**) se o jogo não existe.
- THE SYSTEM SHALL manter `GET /games` (lista) **inalterado** (sem o perfil, para não inflar a listagem).

### Forma do perfil

Um perfil de multiplayer é `{ archetype, topology, params }`:

- `archetype` (string, obrigatório): `"queue_arena"` (único no MVP).
- `topology` (string, obrigatório): `"dedicated"` (único no MVP).
- `params` (objeto, obrigatório): os parâmetros do arquétipo — abaixo, para `queue_arena`.

### Parâmetros do `queue_arena`

| Campo | Tipo | Regra |
|---|---|---|
| `structure` | string | `"ffa"` ou `"teams"` |
| `playerCount` | int | **se `ffa`**: total de jogadores da partida; `≥ 2` |
| `teamCount` | int | **se `teams`**: nº de times; `≥ 2` |
| `teamSize` | int | **se `teams`**: jogadores por time; `≥ 1` |
| `winCondition` | string | `"first_to_score"` \| `"highest_at_time"` \| `"objective"` \| `"elimination"` |
| `scoreTarget` | int | **se `first_to_score`**: pontuação-alvo; `> 0` |
| `durationSeconds` | int | **se `highest_at_time`**: duração; `> 0` |
| `matchStructure` | string | `"single"` ou `"best_of_rounds"` |
| `roundsToWin` | int | **se `best_of_rounds`**: rounds para vencer; `> 0` |
| `allowDraw` | bool | empate permitido no desfecho |
| `scoringStats` | []string | métricas que o resultado carregará (ex.: `["kills","assists"]`); pode ser vazia |

### Validação do `queue_arena` (regras condicionais)

- IF `structure = "ffa"` → THE SYSTEM SHALL exigir `playerCount ≥ 2` e **ignorar/rejeitar** `teamCount`/`teamSize`.
- IF `structure = "teams"` → THE SYSTEM SHALL exigir `teamCount ≥ 2` e `teamSize ≥ 1` e **ignorar/rejeitar** `playerCount`.
- IF `winCondition = "first_to_score"` → THE SYSTEM SHALL exigir `scoreTarget > 0`.
- IF `winCondition = "highest_at_time"` → THE SYSTEM SHALL exigir `durationSeconds > 0`.
- IF `matchStructure = "best_of_rounds"` → THE SYSTEM SHALL exigir `roundsToWin > 0`; para `"single"`, `roundsToWin` é ausente.
- IF `winCondition ∈ {objective, elimination}` → `scoreTarget`/`durationSeconds` são **opcionais** (o desfecho não depende deles).
- WHEN `structure = "teams"` e `winCondition = "objective"` (ou qualquer combinação válida) → THE SYSTEM SHALL aceitar; a **granularidade** do resultado (por time vs. por jogador) é derivada do `structure` pelos serviços consumidores (fatias seguintes), não armazenada aqui.
- Cada item de `scoringStats` SHALL casar `^[a-z][a-z0-9_]*$` (identificador estável); vazio é válido.

## Contratos de Interface

### API HTTP

| Método | Rota | Retorno |
|---|---|---|
| PUT | `/games/{id}/multiplayer-profile` | `200 {perfil}` \| `400` \| `404` |
| GET | `/games/{id}/multiplayer-profile` | `200 {perfil}` \| `404` |
| GET | `/games/{id}` | `200 {..., multiplayerProfile: {perfil}\|null}` |

### GameService (Port de entrada — extensão)

- `SetMultiplayerProfile(ctx, gameID string, p MultiplayerProfile) (*MultiplayerProfile, error)` — upsert; `ErrGameNotFound`, `ErrUnsupportedArchetype`, `ErrUnsupportedTopology`, `ErrInvalidProfileParams`.
- `GetMultiplayerProfile(ctx, gameID string) (*MultiplayerProfile, error)` — `ErrGameNotFound` / `ErrProfileNotFound`.

### GameRepository (Port de saída — extensão)

- `UpsertMultiplayerProfile(ctx, gameID string, p MultiplayerProfile) error` — `ErrGameNotFound` se o jogo não existir.
- `FindMultiplayerProfile(ctx, gameID string) (*MultiplayerProfile, error)` — `ErrProfileNotFound` se ausente.

### Tipos (domínio)

- `MultiplayerProfile{ Archetype string; Topology string; Params QueueArenaParams }` — no MVP `Params` é do `queue_arena`; a forma polimórfica por arquétipo é decisão do **plan** (o domínio não deve assumir um só arquétipo para sempre).
- `QueueArenaParams{ Structure string; PlayerCount int; TeamCount int; TeamSize int; WinCondition string; ScoreTarget int; DurationSeconds int; MatchStructure string; RoundsToWin int; AllowDraw bool; ScoringStats []string }`

## Modelo de Dados

O perfil pertence ao jogo (1:1, opcional). A escolha entre **coluna(s) no `games`**, **tabela dedicada** ou **JSONB** é decisão do **plan** (o JSONB acomoda params polimórficos por arquétipo; uma tabela dedicada é mais explícita). A migration versionada é **task desta feature** (SDD: infra do próprio serviço). Leituras de jogos sem perfil SHALL devolver `multiplayerProfile: null` sem quebrar (compat com registros de `001`).

## Casos de Teste Obrigatórios

- [ ] (service) `SetMultiplayerProfile` válido (`queue_arena`/`dedicated`) → upsert; reconfigurar substitui
- [ ] (service) archetype ≠ `queue_arena` → `ErrUnsupportedArchetype`; topology ≠ `dedicated` → `ErrUnsupportedTopology`
- [ ] (service) `ffa` sem `playerCount ≥ 2` → `ErrInvalidProfileParams`; `teams` sem `teamCount ≥ 2`/`teamSize ≥ 1` → `ErrInvalidProfileParams`
- [ ] (service) `first_to_score` sem `scoreTarget > 0` → inválido; `highest_at_time` sem `durationSeconds > 0` → inválido; `best_of_rounds` sem `roundsToWin > 0` → inválido
- [ ] (service) `scoringStats` com item fora de `^[a-z][a-z0-9_]*$` → inválido; lista vazia → válido
- [ ] (service) perfil para jogo inexistente → `ErrGameNotFound`
- [ ] (repo, integração) upsert round-trip; reconfiguração substitui; `FindMultiplayerProfile` sem perfil → `ErrProfileNotFound` — testcontainers
- [ ] (transport) `PUT .../multiplayer-profile` → 200/400/404 conforme casos; exige internal key
- [ ] (transport) `GET /games/{id}` inclui `multiplayerProfile` (perfil ou `null`); `GET /games` **não** inclui

## Fora do escopo

- **Outros arquétipos** (`battle_royale`, `host_world_dropin`, `coop_mission`) — features futuras.
- **Topologia `p2p`** — reservada (ADR-0023); rejeitada por ora.
- **Consumo do perfil** por matchmaking/session e o **resultado polimórfico** — fatias seguintes.
- **UI de cadastro** (admin-bff/admin-ui) — feature à parte.
- **Versionamento do perfil** (histórico ao mudar em produção) — backlog (ADR-0023 aponta a necessidade).
- **Presets de `mode`** (açúcar que preenche vários params de uma vez) — evolução; aqui os params são explícitos.
