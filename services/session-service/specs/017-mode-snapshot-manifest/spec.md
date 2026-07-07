# Spec: 017-mode-snapshot-manifest

**Serviço:** session-service (+ atualização dos 4 contratos de data plane)
**Status:** Aprovado (2026-07-07)
**Data:** 2026-07-07
**ADR:** 0024 · **Roadmap:** `multiplayer-modes-roadmap.md` — fatia **M1** (núcleo)

## Contexto

O núcleo do M1: a sessão passa a **saber de qual modo nasceu** e a **congelar o perfil** daquele momento. Isso fecha, com um mecanismo só, três lacunas do review do ADR-0023:

1. **Versionamento** — mudar o perfil no registry não afeta sessões em voo (o snapshot é a verdade da sessão).
2. **Integridade** — o `ReportResult` valida o `kind` do outcome contra o **arquétipo do snapshot** (um server de `queue_arena` não consegue mais poluir rollups com `collective`).
3. **Params hardcoded no data plane** — o **session manifest** entrega `{modeId, archetype, topology, params}` ao game server, que lê **só as chaves que conhece** (mata o `first-to-5` fixo; um binário Unity serve N modos).

**Decisão de mecanismo (fixada aqui, não no plan): manifest via RPC `GetSessionConfig` no próprio session.** Alternativa rejeitada: annotations na alocação Agones — só existiria no adapter Agones (o `static` local ficaria sem manifest), acoplaria o formato ao orquestrador e o game server **já fala gRPC com o session** (`ReportResult`) — o manifest usa o mesmo canal e a mesma auth.

Retrocompat: `mode_id` ausente = `default` em todas as entradas (como no matchmaking, spec `010`).

## Comportamento

### Snapshot do perfil na criação

- WHEN `CreateSession(match, game, teams, mode_id?)` → THE SYSTEM SHALL buscar o perfil do **modo** no game-registry (`/multiplayer-profiles/{modeId}`), **congelar o snapshot na sessão** (`ModeID` + perfil + `profileHash`) e seguir o fluxo atual (alocar server, tokens, registrar).
- WHEN `OpenWorld(..., mode_id?)` → idem; os **slots por papel passam a derivar do modo** (hoje derivam do alias legado). Mundos de modos diferentes coexistem no mesmo jogo.
- IF o jogo/modo não existir ou o registry estiver indisponível → a criação **falha** (`ErrProfileUnavailable` → `FailedPrecondition`), como o `OpenWorld` já faz hoje. Cache TTL curto (decorator, como no matchmaking) ameniza: o matchmaking leu o mesmo perfil segundos antes.
- O snapshot é **imutável**: `PUT` no registry depois da criação não muda a sessão.

### Claim `mode_id` no connect token

- THE SYSTEM SHALL emitir o claim **`mode_id`** em todo connect token (queue-form e host_world) — o game server sabe o modo no connection approval sem chamada extra. Sempre presente (inclusive `"default"`), diferente dos claims condicionais `team_id`/`role`.

### Session manifest — `GetSessionConfig`

- WHEN `GetSessionConfig(ref_id)` (novo RPC; `ref_id` = `match_id` **ou** `session_id`, mesma rechaveação do `ReportResult`/host_world) → `200` com o manifest do snapshot:
  `{ session_id, match_id?, game_id, mode_id, archetype, topology, params_json, profile_hash }`
  - `params_json`: os params do arquétipo como **JSON cru** (string) — o data plane desserializa só o que conhece; chave nova no perfil **não muda este contrato**.
  - `profile_hash`: hash (SHA-256 truncado) do JSON canônico do perfil — correlação/log entre control e data plane.
- IF a sessão não existir/estiver fechada → `NotFound`.
- Chamado pelo game server no boot da sessão (mesma auth do `ReportResult`).

### Validação `kind` × arquétipo no `ReportResult`

- THE SYSTEM SHALL rejeitar outcome cujo `kind` não corresponda ao arquétipo do snapshot (`ErrOutcomeMismatch` → `FailedPrecondition`): `queue_arena → win_loss_draw` · `battle_royale → placement` · `coop_mission → collective` · `host_world_dropin → nenhum` (usa `ReportParticipationEvent`; qualquer `ReportResult` é mismatch).
- A validação estrutural existente (spec 009/015/016) permanece; esta é adicional e vem **antes** do publish.

### `modeId` no envelope de fan-out

- THE SYSTEM SHALL incluir `modeId` em **todos** os envelopes do tópico `match-results` (`win_loss_draw`, `placement`, `collective`, `participation_event`). Aditivo — `schemaVersion` continua **2** (consumidores tolerantes a campos novos; sinks só o usarão no M2).

### Board admin (aditivo)

- `GET /admin/sessions` (spec 007/014) passa a expor `modeId` por sessão — o Rooms pode mostrar o modo (surfacing fino fica pro console, pós-M1).

## Contratos de Interface

### gRPC (proto — aditivo, regenerar com `buf`)

| Mudança | Detalhe |
|---|---|
| `CreateSessionRequest.mode_id = 5` | opcional; vazio = `default` |
| `OpenWorldRequest.mode_id = 5` | idem |
| **novo RPC** `GetSessionConfig(GetSessionConfigRequest) returns (GetSessionConfigResponse)` | `GetSessionConfigRequest{ string ref_id = 1 }`; `GetSessionConfigResponse{ session_id, match_id, game_id, mode_id, archetype, topology, params_json, profile_hash }` |

### Internos

- `domain.Session` ganha `ModeID string` + `Profile` (snapshot; tipo espelho local do perfil — o session **não** importa o domain do game-registry, ADR-0002) + `ProfileHash string`.
- `registryclient` do session busca por modo; **decorator de cache TTL** (mesmo padrão do matchmaking, GO-LEARNINGS #21).
- `domain.ConnectClaims` ganha `ModeID`; `ConnectIssuer.Issue` emite o claim.
- Erro novo `ErrOutcomeMismatch`; publishers (`sns`/`log`) ganham `modeId` nos envelopes.

### Docs (parte da fatia)

- Os **4 contratos** `docs/game-models/*-dataplane-contract.md` ganham: claim `mode_id` no token + seção "Session manifest (`GetSessionConfig`)" com o shape e o momento da chamada (boot da sessão) — os data planes Unity novos já nascem lendo modo + manifest; o `queue_arena` ganha o item de migração "trocar `first-to-5` hardcoded pela leitura de `winCondition`/`scoreTarget` do manifest".

## Casos de Teste Obrigatórios

- [ ] (service) `Create`/`OpenWorld` congelam snapshot do modo pedido; `mode_id` vazio = `default`; modo inexistente/registry fora → `ErrProfileUnavailable` (nada alocado)
- [ ] (service) snapshot imutável: mudar o perfil (mock) após criar não altera manifest/validação da sessão em voo
- [ ] (service) `GetSessionConfig` por `match_id` e por `session_id` (host_world); sessão fechada/inexistente → not found; `params_json` decodifica de volta para os params do modo
- [ ] (service) `ReportResult` com kind certo por arquétipo passa; os 3 cruzamentos errados + `ReportResult` em `host_world_dropin` → `ErrOutcomeMismatch` (nada publicado, sessão segue aberta)
- [ ] (token) claim `mode_id` presente nos tokens de `Create` (todos os grants) e `OpenWorld`/`JoinWorld`
- [ ] (publisher) os 4 envelopes carregam `modeId`; `schemaVersion` continua 2
- [ ] (transport gRPC) `mode_id` flui do request; `GetSessionConfig` mapeado; `FailedPrecondition` no mismatch
- [ ] (regressão) suíte atual verde — fluxos sem `mode_id` = comportamento de hoje com modo `default`

## Fora do escopo

- **Segmentação dos sinks por `modeId`** — M2 (gated por evidência).
- **Guarda de modo na injeção** (`JoinWorld` de phantom vindo de outro modo) — follow-up (o filtro do matchmaking cobre o caminho feliz).
- **Persistência durável do snapshot** — sessões seguem em memória (MVP atual); o snapshot vive com a sessão.
- **Implementação Unity** dos contratos (cross-repo; os docs desta fatia são a fonte).
