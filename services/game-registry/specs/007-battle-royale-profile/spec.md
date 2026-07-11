# Spec: 007-battle-royale-profile

**Serviço:** game-registry
**Status:** Done
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo — arquétipo + parâmetros)
**Roadmap:** `docs/game-models/battle_royale-roadmap.md` — fatia **BR0**

## Contexto

Primeira fatia (**BR0**) do arquétipo `battle_royale` (ficha `docs/game-models/03-battle-royale.md`). O game-registry (fonte canônica de jogos, ADR-0017) já guarda um **perfil de multiplayer** polimórfico por arquétipo (spec `006`, `queue_arena`; e o `host_world_dropin` já adicionado depois). Esta feature **adiciona o terceiro arquétipo** — `battle_royale` — ao cadastro: persistir, validar e expor seus parâmetros. É **puramente aditiva**: não muda `queue_arena`/`host_world_dropin`, não toca matchmaking/session/resultado (fatias BR1–BR3), e **não** exige migration (a coluna JSONB `multiplayer_profile` já existe — spec 006, migration `005`).

O `battle_royale` (lobby grande, eliminação progressiva, **colocação ordinal**, respawn configurável) declara aqui só a sua **forma**. Duas fronteiras do roadmap valem como contexto de design:

- **Modo de respawn é comportamento de data plane** (roadmap, decisão #2): `respawnMode` é um param **declarado e armazenado** que o data plane lê; o control plane **não implementa** lógica de respawn. Aqui é só mais um campo string validado por vocabulário.
- **Zona/loot/spawn/eliminação física = data plane** (ficha §6): o perfil não modela nada disso.

Como as demais escritas de perfil, é **control plane interno**, atrás da internal key (ADR-0018).

## Comportamento

### Definir/atualizar o perfil `battle_royale`

- WHEN `PUT /games/{id}/multiplayer-profile` com `archetype = "battle_royale"`, `topology = "dedicated"` e `params` **válidos** → THE SYSTEM SHALL fazer **upsert** do perfil (cria se ausente, substitui se presente) e retornar `200` com o perfil persistido.
- IF o jogo `{id}` não existir → `404` (**ErrGameNotFound**).
- IF a `topology` não for `dedicated` → `400` (**ErrUnsupportedTopology**) (`p2p` segue reservado — ADR-0023).
- IF os `params` do `battle_royale` forem inválidos → `400` (**ErrInvalidProfileParams**), com a razão (ver "Validação").
- THE SYSTEM SHALL continuar aceitando `queue_arena` e `host_world_dropin` **sem regressão** (o `battle_royale` só amplia o conjunto de `archetype` suportados).

### Consultar o perfil

- WHEN `GET /games/{id}` → THE SYSTEM SHALL incluir `multiplayerProfile` com o perfil `battle_royale` quando configurado (mesmo envelope `{archetype, topology, params}` das demais).
- WHEN `GET /games/{id}/multiplayer-profile` → `200` com o perfil, ou `404` (**ErrProfileNotFound**) se sem perfil; `404` (**ErrGameNotFound**) se o jogo não existe.
- `GET /games` (lista) segue **sem** o perfil (inalterado).

### Parâmetros do `battle_royale`

| Campo | Tipo | Regra |
|---|---|---|
| `lobbySize` | int | alvo de jogadores no lobby; `≥ 2` e `≥ 2 × teamSize` (o BR precisa de ao menos dois lados para "último de pé") |
| `teamSize` | int | jogadores por squad; `≥ 1` (`1` = solo/FFA; `2` = duo; `4` = squad) |
| `respawnMode` | string | `"one_life"` \| `"teammate_revive"` \| `"arena_1v1"` \| `"reboot_timer"` |
| `scoringStats` | []string | métricas **extras** que o resultado carregará além da colocação/sobrevivência (ex.: `["kills"]`); pode ser vazia |

> **Nota de modelagem:** a **colocação** (`placement`) e o **tempo de sobrevivência** (`survivedMs`) são intrínsecos ao outcome do `battle_royale` (fatia BR2) — **não** são declarados como `scoringStats`. `scoringStats` cobre só métricas adicionais opcionais (kills, revives…), reusando o mesmo vocabulário do `queue_arena`.

### Validação do `battle_royale` (regras)

- THE SYSTEM SHALL exigir `teamSize ≥ 1`.
- THE SYSTEM SHALL exigir `lobbySize ≥ 2` **e** `lobbySize ≥ 2 × teamSize`.
- THE SYSTEM SHALL exigir `respawnMode ∈ {one_life, teammate_revive, arena_1v1, reboot_timer}`.
- Cada item de `scoringStats` SHALL casar `^[a-z][a-z0-9_]*$` (identificador estável); lista vazia é válida.
- THE SYSTEM SHALL **não** exigir que `lobbySize` seja múltiplo de `teamSize` (squads incompletos são completados com solos pelo matchmaking na BR1).

## Contratos de Interface

### API HTTP

Sem rotas novas — reusa as da spec 006. O `battle_royale` só amplia os valores aceitos em `archetype`/`params`.

| Método | Rota | Retorno |
|---|---|---|
| PUT | `/games/{id}/multiplayer-profile` | `200 {perfil}` \| `400` \| `404` |
| GET | `/games/{id}/multiplayer-profile` | `200 {perfil}` \| `404` |
| GET | `/games/{id}` | `200 {..., multiplayerProfile: {perfil}\|null}` |

### Tipos (domínio) — extensão

- `MultiplayerProfile` ganha o ponteiro `BattleRoyale *BattleRoyaleParams` (tagged union por ponteiro; exatamente o do `Archetype` é não-nil).
- Nova constante `ArchetypeBattleRoyale = "battle_royale"`.
- `BattleRoyaleParams{ LobbySize int; TeamSize int; RespawnMode string; ScoringStats []string }`
- Constantes de vocabulário de respawn: `RespawnOneLife`, `RespawnTeammateRevive`, `RespawnArena1v1`, `RespawnRebootTimer`.

### Persistência (repository) — extensão

- `encodeProfile`/`decodeProfile` ganham o `case domain.ArchetypeBattleRoyale` com um `storedBattleRoyale{ lobbySize, teamSize, respawnMode, scoringStats }`. Sem migration (JSONB existente).

## Modelo de Dados

Reusa a coluna JSONB `multiplayer_profile` (formato `{archetype, topology, params}`). O `params` do `battle_royale` é `{lobbySize, teamSize, respawnMode, scoringStats}`. **Nenhuma** migration nesta feature.

## Casos de Teste Obrigatórios

- [ ] (service) `SetMultiplayerProfile` válido (`battle_royale`/`dedicated`) → upsert; reconfigurar substitui
- [ ] (service) `teamSize < 1` → `ErrInvalidProfileParams`
- [ ] (service) `lobbySize < 2` **ou** `lobbySize < 2×teamSize` → `ErrInvalidProfileParams` (ex.: `lobbySize=6, teamSize=4`)
- [ ] (service) `respawnMode` fora do vocabulário → `ErrInvalidProfileParams`; cada valor válido → aceito
- [ ] (service) `scoringStats` com item fora de `^[a-z][a-z0-9_]*$` → inválido; lista vazia → válido
- [ ] (service) `battle_royale` para jogo inexistente → `ErrGameNotFound`
- [ ] (service) `queue_arena`/`host_world_dropin` continuam válidos (sem regressão)
- [ ] (repo, integração) upsert/`FindMultiplayerProfile` round-trip do `battle_royale`; reconfiguração entre arquétipos substitui — testcontainers
- [ ] (transport) `PUT .../multiplayer-profile` com `battle_royale` → 200/400/404 conforme casos; exige internal key
- [ ] (transport) `GET /games/{id}` inclui o `multiplayerProfile` `battle_royale`

## Fora do escopo

- **Consumo do perfil** por matchmaking (lobby/squads, BR1), session (outcome `placement`, BR2) e sinks (BR3).
- **UI de cadastro** (admin-ui) — spec-companheira (ramo de form condicional, como o `host_world_dropin`); o admin-bff é passthrough cru e **não muda**.
- **Modo de respawn como lógica** — é data plane (decisão #2 do roadmap); aqui só se **armazena/valida** o vocabulário.
- **`scoring` como fórmula de recompensa** (peso colocação × kills) — data plane/economia; o perfil só declara quais `scoringStats` o resultado carrega.
- **Topologia `p2p`** — reservada; rejeitada.
- **Skill/SBMM, região, party** no lobby — follow-up da BR1.
