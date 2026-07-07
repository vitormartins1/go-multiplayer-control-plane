# Spec: 010-mode-scoped-matchmaking

**Serviço:** matchmaking-service
**Status:** Rascunho — aguardando aprovação
**Data:** 2026-07-07
**ADR:** 0024 · **Roadmap:** `multiplayer-modes-roadmap.md` — fatia **M1** (lado matchmaking)

## Contexto

O M0 (na main) fez o jogo ter **1—N modos nomeados**; o matchmaking ainda resolve a formação pelo **alias legado** (`GET .../multiplayer-profile` = modo `default`) e agrupa a fila só por `gameID`. Esta fatia faz o **modo fluir na entrada**: o ticket declara (opcionalmente) o modo, a formação é derivada do perfil **daquele modo**, a fila nunca mistura modos e o `mode_id` segue para o session (que o congela — spec `session/017`).

Princípio de retrocompat (ADR-0024): `mode_id` **ausente/vazio = `default`** em todas as entradas — clientes atuais (incluindo o data plane `queue_arena`) continuam funcionando sem mudança.

## Comportamento

### Fila por (jogo, modo) — queue-form (`queue_arena`, `battle_royale`, `coop_mission`)

- WHEN `FindMatch(game_id, mode_id?)` → THE SYSTEM SHALL resolver a formação pelo perfil do **modo** (`GET /games/{id}/multiplayer-profiles/{modeId}` no game-registry) e enfileirar o ticket com `(gameID, modeID)`.
- IF `mode_id` ausente/vazio → usar `default` (mesmo caminho, sem ramo especial além da normalização na borda).
- IF o jogo/modo não existir, o perfil for de arquétipo sem fila (`host_world_dropin`) ou o registry estiver fora → o ticket **não entra na fila** (`ErrProfileUnavailable` → `FAILED`, comportamento atual, agora por modo).
- THE SYSTEM SHALL formar matches **apenas entre tickets do mesmo `(gameID, modeID)`** — modos diferentes do mesmo jogo nunca se misturam (extensão da regra multi-tenant do ADR-0017).
- WHEN o match forma → `CreateSession` recebe o **`mode_id`** junto de match/game/teams (campo novo, spec `session/017`).
- O cache de formação (`profilecache`) passa a ser chaveado por `(gameID, modeID)` — TTL e semântica inalterados.

### Entradas do `host_world_dropin` (invasão e sinais)

- `Invade`, `PostSummonSign` e `ListSigns` ganham `mode_id` opcional (ausente = `default`): as políticas (`InvasionPolicy`/`CoopPolicy`) passam a ser lidas do perfil do **modo**.
- O sinal registrado no quadro carrega o `modeID`; `ListSigns` devolve **apenas sinais do mesmo modo** do host — um aliado anunciado no modo X não aparece para um host no modo Y.
- `SummonSign`/injeção seguem inalterados (a injeção é por `session_id`; a coerência de modo vem do filtro do `ListSigns` — guarda dura no session fica fora do M1, ver Fora do escopo).

## Contratos de Interface

### gRPC (proto — campos aditivos, números novos)

| Message | Campo novo |
|---|---|
| `FindMatchRequest` | `string mode_id = 2` (opcional; vazio = `default`) |
| `InvadeRequest` | `string mode_id = 4` |
| `PostSummonSignRequest` | `string mode_id = 4` |
| `ListSignsRequest` | `string mode_id = 4` |

Regenerar com `buf` (gotcha do `rawDesc`). `TicketUpdate`/respostas não mudam.

### Internos

- `domain.ProfileProvider.Format(ctx, gameID, modeID)` (assinatura ganha o modo); idem `InvasionPolicyProvider`/`CoopPolicyProvider`.
- `registryclient` busca `/games/{id}/multiplayer-profiles/{modeId}`; **deixa de usar o alias legado**.
- `ticket` ganha `modeID`; `tryForm` agrupa por chave composta `gameID+"\x00"+modeID` (ou struct); `provisioner.Provision` ganha `modeID` e o `sessionclient` o repassa no `CreateSessionRequest`.
- Sinal do quadro (`summoner`) ganha `modeID`; `ListSigns` filtra por ele.
- Normalização (`"" → "default"`) acontece **uma vez, na borda** (transport), não espalhada.

## Casos de Teste Obrigatórios

- [ ] (service) tickets do mesmo jogo em **modos diferentes** nunca formam match juntos; mesmo modo forma normalmente
- [ ] (service) `mode_id` vazio = `default` (ticket agrupa com quem pediu `default` explícito)
- [ ] (service) modo inexistente → `ErrProfileUnavailable` (ticket rejeitado); modo existente de outro arquétipo com fila funciona (ex.: `default`=queue_arena e `warzone`=battle_royale no mesmo jogo, filas independentes)
- [ ] (registryclient) `Format` chama a URL da coleção com o modeId; cache por `(game, mode)` — dois modos do mesmo jogo não colidem no cache
- [ ] (service) `Provision` recebe e repassa o `modeID` (mock do sessionclient)
- [ ] (invade/sinais) política lida do modo; `ListSigns` só devolve sinais do mesmo modo
- [ ] (regressão) suíte atual verde — fluxos sem `mode_id` idênticos ao comportamento de hoje

## Fora do escopo

- **Snapshot/validação/manifest/token/envelope** — lado session (spec `017`).
- **Guarda dura de modo na injeção** (`JoinWorld` rejeitar phantom de modo diferente) — o filtro do `ListSigns`/`Invade` cobre o caminho feliz; guarda no session é follow-up.
- **Filas ponderadas/skill/região** — ortogonais (log de generalizações).
- Remoção do alias legado no registry (pós-M1).
