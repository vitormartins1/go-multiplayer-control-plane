# Spec: 007-time-windows

**Serviço:** leaderboard-service
**Status:** Rascunho
**Data:** 2026-07-08

## Contexto

Fecha o backlog "leaderboard: seasons/janelas por período + paginação". Hoje cada `(game, board)` é **um** sorted set all-time: quem entrou cedo domina o topo para sempre e não há como responder "quem foi melhor esta semana?". Esta fatia adiciona **janelas derivadas do relógio** (diária/semanal/mensal, além do all-time) e **paginação por offset** nas leituras.

Princípios da fatia:

- **Janela derivada do relógio (UTC), zero configuração:** `daily`/`weekly`/`monthly` são calculadas do timestamp do write — nenhum cadastro, nenhum estado de "season ativa". **Seasons nomeadas** (rollover manual, config por jogo/modo) ficam **adiadas por evidência**: exigem decidir onde vive a config (game-registry?) — ADR próprio quando houver demanda real.
- **All-time é a key existente** — zero migration, leituras atuais continuam válidas; `window` ausente = all-time (compat total para todos os consumidores atuais, incluindo o sink e o painel).
- **Write espalha, read aponta:** todo write (highscore e cumulativo) alimenta as 4 janelas; a leitura escolhe uma. Consumidores não mudam.
- Keys periódicas têm **TTL** (limpeza automática; a janela corrente nunca expira no meio — ver plan).

## Comportamento

### Janelas

- THE SYSTEM SHALL manter, por `(game, board)`, 4 rankings: `alltime` (key atual), `daily`, `weekly` (semana ISO) e `monthly`, derivados do relógio UTC no momento do write.
- WHEN `Submit`/`ForceSubmit` (highscore) → THE SYSTEM SHALL aplicar o score nas 4 janelas (ZADD GT nas 4; force = ZADD sem GT nas 4).
- WHEN `ApplyCumulative` (sink) → THE SYSTEM SHALL somar as parcelas nas 4 janelas **na mesma operação atômica e idempotente** por `matchID` (guarda única — reprocessar não soma em nenhuma janela).
- WHEN leitura (`Top`/`Around`/`me`) com `?window=daily|weekly|monthly|alltime` → THE SYSTEM SHALL responder do ranking da **janela corrente** correspondente; ausente → `alltime`.
- IF `window` inválida → `400` (**ErrInvalidWindow**).
- A entry devolvida pelo `Submit` (rank pós-submissão) segue sendo a do **all-time** (contrato atual inalterado).

### Paginação

- WHEN `GET /leaderboard` ou `GET /admin/games/{id}/leaderboard` com `?offset=N` → THE SYSTEM SHALL devolver a página `[offset, offset+limit)` (rank 1-based contínuo: a página 2 começa no rank offset+1).
- `offset` ausente/inválido/negativo → 0. `limit` mantém o clamp atual (default 10, máx. existente).

### Casos de borda

- Partida processada perto da virada de período: as parcelas caem nas janelas do **momento do processamento** (at-least-once + guarda de idempotência = uma aplicação só; a janela é a do primeiro processamento). Aceito — sem relógio da partida no envelope.
- Janela periódica sem nenhum score → resposta vazia (mesma forma de board vazio hoje).

## Contratos de Interface

### API HTTP (player e admin)

| Endpoint | Novos params | Retorno |
|---|---|---|
| `GET /leaderboard` (JWT) | `window`, `offset` | `200 {entries}` \| `400` (board/window) |
| `GET /leaderboard/me` (JWT) | `window` | idem hoje |
| `GET /admin/games/{gameId}/leaderboard` | `window`, `offset` | idem player |
| `POST /scores`, `POST|PUT /admin/.../scores` | — (writes espalham sozinhos) | inalterado |

### Tipos (domínio) — extensão

- Constantes `WindowAllTime|WindowDaily|WindowWeekly|WindowMonthly` + `ValidWindow(w)`; erro novo `ErrInvalidWindow`.
- `LeaderboardService`: `Top`/`Around` ganham `window string`; `Submit`/`ForceSubmit`/`ApplyCumulative` mantêm assinatura (espalham internamente).
- `LeaderboardStore`: leituras (`Rank`/`Top`/`Range`) ganham `window`; writes mantêm assinatura e espalham nas 4 janelas.

## Modelo de Dados (Redis)

```
<prefix>:<gameID>:<board>              ← alltime (key atual, intacta)
<prefix>:<gameID>:<board>:d:20260708   ← daily   (TTL ~3 dias)
<prefix>:<gameID>:<board>:w:2026W28    ← weekly  (TTL ~15 dias)
<prefix>:<gameID>:<board>:m:202607     ← monthly (TTL ~45 dias)
<prefix>:processed:<gameID>            ← guarda de idempotência (inalterada, única p/ todas as janelas)
```

## Casos de Teste Obrigatórios

- [ ] (service) `Top`/`Around` validam window (`ErrInvalidWindow`); ausente → alltime; offset negativo → 0
- [ ] (store, integração/miniredis ou mock conforme harness atual) write espalha nas 4 keys; leitura por janela lê a key certa; TTL setado nas periódicas e ausente na all-time
- [ ] (store) `ApplyCumulative`: reprocessar o mesmo matchID é no-op em TODAS as janelas (guarda única)
- [ ] (store) paginação: `Range` com offset devolve ranks contínuos (offset+1…)
- [ ] (transport) `?window` inválida → 400; `?window=weekly` + `?offset` propagados; defaults preservados (respostas atuais byte-idênticas sem os params novos)
- [ ] (consumer) sem mudança de código — compilação prova; teste existente segue verde

## Fora do escopo

- **Seasons nomeadas** (rollover manual, premiação, config por jogo/modo) — adiadas por evidência; exigirão ADR (onde vive a config) quando houver demanda.
- **Consulta a períodos passados** (`?date=`) — as keys periódicas retidas pelo TTL deixam a porta aberta; expor é fatia futura.
- **Segmentação por modo (M2 do ADR-0024)** — continua gated; quando vier, `modeId` entra como dimensão na key, ortogonal à janela.
- **BFF e console** — specs companheiras `admin-bff/019-leaderboard-windows` e `admin-ui/039-leaderboard-windows`.
