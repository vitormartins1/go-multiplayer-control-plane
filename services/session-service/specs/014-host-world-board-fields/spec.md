# Spec: 014-host-world-board-fields

**Serviço:** session-service
**Status:** Done (2026-07-07)
**Data:** 2026-07-07
**ADR:** 0023 — **arquétipo `host_world_dropin`, fatia H3** (surfacing no console — lado session)
**Roadmap:** `docs/game-models/host_world_dropin-roadmap.md` → H3
**Par de:** `admin-bff/014-host-world-rooms`, `admin-ui/020-host-world-console`
**Depende de:** `session/011` (mundo host-cêntrico, slots) e o board admin (`session/007`).

## Contexto

O board admin (`session/007`) expõe as sessões via `GET /admin/sessions` (e `/admin/games/{id}/sessions`), mas o `sessionDTO` carrega o **mínimo do queue_arena**: `players`, `state`, `serverAddress`, tempos. Um mundo `host_world_dropin` tem estrutura que o board não vê: **origem** (é mundo, não match), **host** e **slots por papel** (co-op X/2, invasor Y/1). Sem esses campos, o console (H3) mostra "N players" para um mundo host-cêntrico — perde o modelo.

Esta fatia é **puramente aditiva de serialização**: o `Session` (domínio) já tem `Origin`, `HostPlayerID` e `RoleSlots` (H0); o `sessionDTO` do HTTP admin passa a expô-los. Nenhuma regra nova, nenhuma mudança no gRPC.

## Comportamento

- WHEN o board admin lista sessões (`GET /admin/sessions` ou `/admin/games/{id}/sessions`) → THE SYSTEM SHALL incluir em cada sessão: `origin` (`"match"` | `"host_world"`), `hostPlayerId` (só em `host_world`; vazio/omitido em `queue_arena`) e `roleSlots` (só em `host_world`): a lista de papéis com `capacity` e `occupied`.
- THE SYSTEM SHALL manter todos os campos atuais (`players`, `state`, etc.) inalterados — `queue_arena` não muda de forma observável além dos campos novos (que ficam vazios/omitidos).
- THE SYSTEM SHALL ordenar `roleSlots` de forma **estável** (ex.: `coop` antes de `invader`) para o JSON ser determinístico.

## Contratos de Interface

### HTTP (`sessionDTO`)

Campos novos (aditivos):

```json
{
  "origin": "host_world",
  "hostPlayerId": "player-1",
  "roleSlots": [
    { "role": "coop", "capacity": 2, "occupied": 1 },
    { "role": "invader", "capacity": 1, "occupied": 0 }
  ]
}
```

`origin` sempre presente; `hostPlayerId`/`roleSlots` `omitempty` (ausentes no `queue_arena`).

- Nada no gRPC/proto. Nada no domínio (os campos já existem). Só o `toSessionDTO`.

## Modelo de Dados

Sem mudança (sessão em memória; `Origin`/`HostPlayerID`/`RoleSlots` já no `Session`). Sem migration.

## Fora de escopo

- Quadro de sinais de co-op ativo por mundo — follow-up (não há leitura no matchmaking; ver `host_world_dropin-follow-ups.md`).
- Qualquer mudança no gRPC (`OpenWorld`/`JoinWorld`/…).

## Critérios de Aceite

- [ ] `host_world`: `origin="host_world"`, `hostPlayerId` preenchido, `roleSlots` com `capacity`/`occupied` por papel, ordenado.
- [ ] `queue_arena`: `origin="match"`, sem `hostPlayerId`/`roleSlots`; demais campos idênticos.
- [ ] Testes do handler (bufconn/httptest) cobrindo os dois; `-race` verde; sem regressão do board.
