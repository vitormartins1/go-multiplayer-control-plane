# Spec: 009-multiplayer-modes

**Serviço:** game-registry
**Status:** Rascunho — aguardando aprovação
**Data:** 2026-07-07
**ADR:** 0024 (modos de jogo — perfis nomeados + session manifest)
**Roadmap:** `docs/game-models/multiplayer-modes-roadmap.md` — fatia **M0** (lado game-registry)

## Contexto

Primeira fatia (**M0**) do ADR-0024. Hoje a relação jogo↔perfil é **1—1** (coluna JSONB `multiplayer_profile`, specs `006`–`008`), mas jogo real = **conjunto de modos** (Elden Ring = `host_world_dropin` + `queue_arena` no Coliseu; CoD = `queue_arena` + `battle_royale`; TDM e S&D = dois `queue_arena` com params diferentes). Esta feature transforma o perfil em **coleção de perfis nomeados** — cada modo com `modeId` (slug estável), `displayName` opcional e o corpo **inalterado** do ADR-0023 (`archetype`, `topology`, `params`, com a validação por arquétipo **reusada intacta**).

Princípios da fatia:

- **Puramente aditiva para os consumidores atuais:** o endpoint `multiplayer-profile` (singular) vira **alias do modo `default`** — matchmaking (`registryclient`) e session continuam funcionando sem mudança até o M1.
- **Migração lazy, sem migration SQL:** a coluna JSONB existente passa a guardar um **mapa de modos**; um valor no formato antigo (objeto com `archetype` no topo) é lido como `{default: <objeto>}` e reescrito no formato novo no próximo write.
- **`modeId` é identidade estável** (vai para ticket/token/envelope no M1): slug imutável na prática — renomear a exibição é `displayName`, não o id.
- Como as demais escritas de perfil: **control plane interno**, atrás da internal key (ADR-0018).

## Comportamento

### Coleção de modos

- WHEN `PUT /games/{id}/multiplayer-profiles/{modeId}` com corpo `{displayName?, archetype, topology, params}` **válido** → THE SYSTEM SHALL fazer **upsert** do modo e retornar `200` com o modo persistido (`{modeId, displayName, archetype, topology, params}`).
- WHEN `GET /games/{id}/multiplayer-profiles` → `200` com a lista de modos (`default` primeiro, demais por `modeId` asc). Jogo sem nenhum modo → `200` com lista vazia.
- WHEN `GET /games/{id}/multiplayer-profiles/{modeId}` → `200` com o modo, ou `404` (**ErrModeNotFound**).
- WHEN `DELETE /games/{id}/multiplayer-profiles/{modeId}` → `204`; IF `modeId = "default"` → `409` (**ErrDefaultModeProtected** — o `default` é o fallback dos consumidores até o M1); IF o modo não existir → `404` (**ErrModeNotFound**).
- IF o jogo `{id}` não existir → `404` (**ErrGameNotFound**) em todas as rotas.
- IF `modeId` inválido (ver regras) → `400` (**ErrInvalidModeID**).
- IF o jogo já tiver **16 modos** e o `PUT` criar um novo → `400` (**ErrTooManyModes**).

### Alias de compatibilidade (endpoint singular — inalterado por fora)

- WHEN `PUT /games/{id}/multiplayer-profile` → THE SYSTEM SHALL operar como `PUT .../multiplayer-profiles/default` (mesmo corpo de antes, sem `displayName`), preservando contrato e códigos de erro das specs `006`–`008`.
- WHEN `GET /games/{id}/multiplayer-profile` → devolve o modo `default` (formato antigo `{archetype, topology, params}`), ou `404` (**ErrProfileNotFound**) se não houver modo `default`.
- WHEN `GET /games/{id}` → segue incluindo `multiplayerProfile` (= modo `default`, ou `null`) **e** passa a incluir `multiplayerModes: [{modeId, displayName, archetype}]` (resumo, ordenado como a lista). `GET /games` (lista) segue **sem** perfil/modos.

### Validação

- `modeId` SHALL casar `^[a-z][a-z0-9-]{0,31}$` (slug estável; `default` é um valor comum, sem tratamento especial além do DELETE).
- `displayName` é opcional; se presente, trimmed, 1–60 caracteres.
- O corpo `{archetype, topology, params}` de **cada modo** passa pela validação por arquétipo **existente** (specs `006`–`008`), sem alteração: `ErrUnsupportedArchetype`/`ErrUnsupportedTopology`/`ErrInvalidProfileParams` idênticos.

## Contratos de Interface

### API HTTP

| Método | Rota | Retorno |
|---|---|---|
| GET | `/games/{id}/multiplayer-profiles` | `200 [modo…]` \| `404` (jogo) |
| PUT | `/games/{id}/multiplayer-profiles/{modeId}` | `200 {modo}` \| `400` \| `404` (jogo) |
| GET | `/games/{id}/multiplayer-profiles/{modeId}` | `200 {modo}` \| `404` |
| DELETE | `/games/{id}/multiplayer-profiles/{modeId}` | `204` \| `404` \| `409` (default) |
| PUT/GET | `/games/{id}/multiplayer-profile` | **alias do `default`** — contrato das specs 006–008 preservado |
| GET | `/games/{id}` | `200 {..., multiplayerProfile, multiplayerModes: [resumo…]}` |

### Tipos (domínio) — extensão

- Novo `MultiplayerMode{ ModeID string; DisplayName string; Profile MultiplayerProfile }` (o `MultiplayerProfile` do ADR-0023 **não muda**).
- `GameService` ganha: `SetMode`, `GetMode`, `ListModes`, `DeleteMode` (assinaturas no plan). `SetMultiplayerProfile`/`GetMultiplayerProfile` passam a **delegar** para o modo `default` (mantidos como métodos — os consumidores internos atuais não mudam).
- Erros novos: `ErrInvalidModeID`, `ErrModeNotFound`, `ErrDefaultModeProtected`, `ErrTooManyModes`.

### Persistência (repository) — extensão

- A coluna JSONB `multiplayer_profile` passa a guardar `{"modes": {"<modeId>": {displayName?, archetype, topology, params}}}`.
- **Upgrade lazy no decode:** valor com `archetype` no topo (formato v1) é interpretado como `{"modes": {"default": <valor>}}`. Todo write persiste o formato novo. **Nenhuma migration SQL.**
- `Game.MultiplayerProfile` (carregado no `FindByID`) segue apontando para o modo `default` (compat interna); `FindByID` também carrega a coleção.

## Modelo de Dados

Mesma coluna `multiplayer_profile JSONB`. Exemplo (jogo com dois modos cruzando arquétipos):

```json
{
  "modes": {
    "default":  { "displayName": "Team Deathmatch", "archetype": "queue_arena",   "topology": "dedicated", "params": { "structure": "teams", "teamCount": 2, "teamSize": 4, "winCondition": "first_to_score", "scoreTarget": 50, "matchStructure": "single", "allowDraw": false, "scoringStats": ["kills"] } },
    "warzone":  { "displayName": "Warzone",         "archetype": "battle_royale", "topology": "dedicated", "params": { "lobbySize": 60, "teamSize": 3, "respawnMode": "reboot_timer", "scoringStats": ["kills"] } }
  }
}
```

## Casos de Teste Obrigatórios

- [ ] (service) `SetMode` válido cria; repetir substitui (upsert); `ListModes` ordena `default` primeiro
- [ ] (service) `modeId` fora do regex → `ErrInvalidModeID`; 17º modo → `ErrTooManyModes`
- [ ] (service) `DeleteMode("default")` → `ErrDefaultModeProtected`; modo inexistente → `ErrModeNotFound`; delete válido remove
- [ ] (service) validação por arquétipo reusada: params inválidos em qualquer modo → `ErrInvalidProfileParams` (um caso por arquétipo basta)
- [ ] (service) `SetMultiplayerProfile`/`GetMultiplayerProfile` (legado) equivalem a operar o modo `default` — **sem regressão** nos casos das specs 006–008
- [ ] (repo, integração) round-trip do formato novo; **upgrade lazy**: valor v1 gravado à mão é lido como `default` e reescrito como v2 no próximo write — testcontainers
- [ ] (transport) CRUD da coleção → 200/204/400/404/409 conforme casos; exige internal key
- [ ] (transport) `GET /games/{id}` inclui `multiplayerProfile` (default) + `multiplayerModes` (resumo); jogo sem modos → `multiplayerProfile: null` + lista vazia

## Fora do escopo

- **`mode_id` no matchmaking/session/token/envelope e o session manifest** — fatia **M1** (`matchmaking/010` + `session/017`).
- **Segmentação de sinks por modo** — fatia **M2** (gated por evidência).
- **UI do console** (lista de modos + form por modo) — spec companheira do admin-ui após aprovação desta; admin-bff é passthrough cru (`json.RawMessage`) e provavelmente não muda.
- **Renomear `modeId`** — id é estável; muda-se `displayName`. Sem endpoint de rename de id.
- **Remoção do alias `multiplayer-profile`** — só depois do M1 (Aberturas do roadmap).
