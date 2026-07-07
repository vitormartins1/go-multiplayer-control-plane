# Spec: 008-coop-mission-profile

**Serviço:** game-registry
**Status:** Aprovado
**Data:** 2026-07-07
**ADR:** 0023 (perfil de multiplayer por jogo — arquétipo + parâmetros)
**Roadmap:** `docs/game-models/coop_mission-roadmap.md` — fatia **C0**

## Contexto

Primeira fatia (**C0**) do arquétipo `coop_mission` (ficha `docs/game-models/04-coop-pve-mission.md`) — o **quarto e último** arquétipo do ADR-0023. O game-registry (fonte canônica de jogos, ADR-0017) já guarda um **perfil de multiplayer** polimórfico por arquétipo (`queue_arena` spec `006`, `host_world_dropin`, `battle_royale` spec `007`). Esta feature **adiciona o quarto arquétipo** — `coop_mission` — ao cadastro: persistir, validar e expor seus parâmetros. É **puramente aditiva**: não muda os outros três arquétipos, não toca matchmaking/session/resultado (fatias C1–C3), e **não** exige migration (a coluna JSONB `multiplayer_profile` já existe — spec 006, migration `005`).

O `coop_mission` (time pequeno vs. ambiente/IA, término por objetivo/wipe, **resultado coletivo** `MISSION_SUCCESS/FAILURE`) declara aqui só a sua **forma**. Fronteiras do roadmap que valem como contexto de design:

- **Missão, dificuldade e bot-fill são comportamento de data plane** (roadmap, decisão #2): `missions`/`difficulties`/`botfillEnabled` são params **declarados e armazenados** que o data plane lê; o control plane **não implementa** seleção de missão nem spawn de bot. Aqui são só campos validados (mesmo tratamento do `respawnMode` do `battle_royale`).
- **IA/Director, hordas, revive, procedural = data plane** (ficha §6): o perfil não modela nada disso.
- **O resultado coletivo é da fatia C2** (session): aqui o perfil não modela o outcome, só declara `scoringStats` (métricas que o resultado carregará).

Como as demais escritas de perfil, é **control plane interno**, atrás da internal key (ADR-0018).

## Comportamento

### Definir/atualizar o perfil `coop_mission`

- WHEN `PUT /games/{id}/multiplayer-profile` com `archetype = "coop_mission"`, `topology = "dedicated"` e `params` **válidos** → THE SYSTEM SHALL fazer **upsert** do perfil (cria se ausente, substitui se presente) e retornar `200` com o perfil persistido.
- IF o jogo `{id}` não existir → `404` (**ErrGameNotFound**).
- IF a `topology` não for `dedicated` → `400` (**ErrUnsupportedTopology**) (`p2p` segue reservado — ADR-0023).
- IF os `params` do `coop_mission` forem inválidos → `400` (**ErrInvalidProfileParams**), com a razão (ver "Validação").
- THE SYSTEM SHALL continuar aceitando `queue_arena`, `host_world_dropin` e `battle_royale` **sem regressão** (o `coop_mission` só amplia o conjunto de `archetype` suportados).

### Consultar o perfil

- WHEN `GET /games/{id}` → THE SYSTEM SHALL incluir `multiplayerProfile` com o perfil `coop_mission` quando configurado (mesmo envelope `{archetype, topology, params}` das demais).
- WHEN `GET /games/{id}/multiplayer-profile` → `200` com o perfil, ou `404` (**ErrProfileNotFound**) se sem perfil; `404` (**ErrGameNotFound**) se o jogo não existe.
- `GET /games` (lista) segue **sem** o perfil (inalterado).

### Parâmetros do `coop_mission`

| Campo | Tipo | Regra |
|---|---|---|
| `teamSize` | int | jogadores no time co-op; `≥ 2` (co-op precisa de ≥ 2; ficha usa ~4, bots preenchem vagas humanas) |
| `missions` | []string | catálogo de missões que o host pode escolher; identificadores estáveis; **pode ser vazia** (data plane) |
| `difficulties` | []string | catálogo de dificuldades (Hazard Level etc.); identificadores estáveis; **pode ser vazia** (data plane) |
| `botfillEnabled` | bool | se vagas humanas são preenchidas por bots (comportamento de data plane) |
| `scoringStats` | []string | métricas **extras** que o resultado carregará (ex.: `["kills","revives"]`) e que **modulam recompensa, não o desfecho**; pode ser vazia |

> **Nota de modelagem:** o **desfecho coletivo** (`MISSION_SUCCESS`/`MISSION_FAILURE`) é intrínseco ao outcome do `coop_mission` (fatia C2, session) — **não** é declarado no perfil. `scoringStats` cobre só métricas por jogador, reusando o mesmo vocabulário do `queue_arena`/`battle_royale`.

### Validação do `coop_mission` (regras)

- THE SYSTEM SHALL exigir `teamSize ≥ 2`.
- Cada item de `missions` SHALL casar `^[a-z][a-z0-9_]*$` (identificador estável) e **não repetir**; lista vazia é válida.
- Cada item de `difficulties` SHALL casar `^[a-z][a-z0-9_]*$` e **não repetir**; lista vazia é válida.
- Cada item de `scoringStats` SHALL casar `^[a-z][a-z0-9_]*$`; lista vazia é válida (mesma convenção dos outros arquétipos — sem checagem de duplicata).

## Contratos de Interface

### API HTTP

Sem rotas novas — reusa as da spec 006. O `coop_mission` só amplia os valores aceitos em `archetype`/`params`.

| Método | Rota | Retorno |
|---|---|---|
| PUT | `/games/{id}/multiplayer-profile` | `200 {perfil}` \| `400` \| `404` |
| GET | `/games/{id}/multiplayer-profile` | `200 {perfil}` \| `404` |
| GET | `/games/{id}` | `200 {..., multiplayerProfile: {perfil}\|null}` |

### Tipos (domínio) — extensão

- `MultiplayerProfile` ganha o ponteiro `CoopMission *CoopMissionParams` (tagged union por ponteiro; exatamente o do `Archetype` é não-nil).
- Nova constante `ArchetypeCoopMission = "coop_mission"`.
- `CoopMissionParams{ TeamSize int; Missions []string; Difficulties []string; BotfillEnabled bool; ScoringStats []string }`

### Persistência (repository) — extensão

- `encodeProfile`/`decodeProfile` ganham o `case domain.ArchetypeCoopMission` com um `storedCoopMission{ teamSize, missions, difficulties, botfillEnabled, scoringStats }`. Sem migration (JSONB existente).

## Modelo de Dados

Reusa a coluna JSONB `multiplayer_profile` (formato `{archetype, topology, params}`). O `params` do `coop_mission` é `{teamSize, missions, difficulties, botfillEnabled, scoringStats}`. **Nenhuma** migration nesta feature.

## Casos de Teste Obrigatórios

- [ ] (service) `SetMultiplayerProfile` válido (`coop_mission`/`dedicated`) → upsert; reconfigurar substitui
- [ ] (service) `teamSize < 2` → `ErrInvalidProfileParams`
- [ ] (service) `missions` com item fora de `^[a-z][a-z0-9_]*$` → inválido; com duplicata → inválido; lista vazia → válido
- [ ] (service) `difficulties` com item fora do regex → inválido; com duplicata → inválido; lista vazia → válido
- [ ] (service) `scoringStats` com item fora do regex → inválido; lista vazia → válido
- [ ] (service) `coop_mission` para jogo inexistente → `ErrGameNotFound`
- [ ] (service) `queue_arena`/`host_world_dropin`/`battle_royale` continuam válidos (sem regressão)
- [ ] (repo, integração) upsert/`FindByID` round-trip do `coop_mission`; reconfiguração entre arquétipos substitui e limpa o ponteiro antigo — testcontainers
- [ ] (transport) `PUT .../multiplayer-profile` com `coop_mission` → 200/400/404 conforme casos; exige internal key
- [ ] (transport) `GET /games/{id}` inclui o `multiplayerProfile` `coop_mission`

## Fora do escopo

- **Consumo do perfil** por matchmaking (formação do time, C1), session (outcome `collective`, C2) e sinks (C3).
- **UI de cadastro** (admin-ui) — companheira nesta mesma fatia C0 (ramo de form condicional + union em `api.ts`), coberta pelas tasks T13; o admin-bff é passthrough cru e **não muda**.
- **Missão/dificuldade/bot-fill como lógica** — é data plane (decisão #2 do roadmap); aqui só se **armazena/valida**.
- **Join-in-progress, lobby/browser, missão+dificuldade na sessão** — Aberturas do roadmap (follow-up; partilham gap do `host_world_dropin`).
- **Topologia `p2p`** — reservada; rejeitada.
