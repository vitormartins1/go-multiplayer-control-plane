# Plan: 007-time-windows

**Spec:** `spec.md`

## Decisões técnicas

1. **Janela = sufixo na key; all-time = key atual (zero migration).**
   `windowKey(gameID, board, window, t)`: `alltime` devolve a key existente; `daily` → `:d:YYYYMMDD`, `weekly` → `:w:YYYY'W'ww` (`time.Time.ISOWeek`, zero-padded), `monthly` → `:m:YYYYMM`. Sempre UTC. O período é derivado do **momento do write** — nenhuma config, nenhum estado.

2. **Relógio injetável no store.**
   `RedisStore` ganha `now func() time.Time` (default `time.Now`; setter/opção para os testes fixarem o período e testarem virada de janela). Evita flakiness perto da meia-noite UTC.

3. **Highscore espalha por pipeline; cumulativo espalha dentro do script.**
   - `Submit`/`ForceScore`: pipeline com 4 `ZADD` (GT ou não) + `EXPIRE` nas 3 keys periódicas. `ZADD GT` é idempotente por natureza — pipeline não-atômico entre keys é aceitável (retry converge).
   - `ApplyCumulative`: o store expande cada `Increment` nas 4 boardKeys e o `applyCumulativeScript` existente já itera trios genéricos — só cresce o `ARGV` (guarda `SADD` única e inalterada: reprocessar é no-op em TODAS as janelas de uma vez). `EXPIRE` das periódicas entra no próprio script (atômico com os ZINCRBYs).

4. **TTL sliding nas keys periódicas: daily 3d, weekly 15d, monthly 45d (consts).**
   O TTL só limpa memória — a correção vem do nome da key (período no sufixo), então renovar o TTL a cada write é inofensivo e simples (sem `EXPIRE NX`). Retenção ≥ 1 período anterior deixa a porta aberta para "consultar período passado" (fatia futura). All-time sem TTL.

5. **Port: leituras ganham `window`; writes não.**
   `LeaderboardStore.Rank/Top/Range` e `LeaderboardService.Top/Around` recebem `window string`. Validação (`ValidWindow`, `ErrInvalidWindow`) no **service**; o store confia. `Submit` continua devolvendo a entry do **all-time** (store.Rank com `WindowAllTime`) — contrato atual intacto.

6. **Paginação por offset no `Top`.**
   `Top(ctx, gameID, board, window, limit, offset)`: service clampa `offset < 0 → 0` e delega para `store.Range(start=offset, stop=offset+limit-1)` (já existe). Transport: `?offset` (Atoi, erro → 0). Respostas sem os params novos ficam byte-idênticas às atuais.

7. **Transport.**
   `windowFromQuery` (ausente → `WindowAllTime`) nos 3 GETs; `?offset` em `/leaderboard` e `/admin/.../leaderboard`. Mapeamento novo: `ErrInvalidWindow` → 400 `invalid_window`.

## Arquivos

| Arquivo | Mudança |
|---|---|
| `internal/domain/leaderboard.go` | consts `Window*`, `ValidWindow`; Ports com `window` nas leituras |
| `internal/domain/errors.go` | `ErrInvalidWindow` |
| `internal/service/leaderboard_service.go` | valida window, clampa offset, repassa |
| `internal/storage/redis_store.go` | `windowKey`, `now` injetável, pipeline no Submit/Force, script com EXPIRE, TTL consts |
| `internal/transport/http_handler.go` | `windowFromQuery`/`offset`, error mapping |
| `cmd/consumer/main.go` | **sem mudança** (writes espalham no store) |
| testes dos 3 níveis | casos da spec; storage via testcontainers com relógio fixado |

## Riscos / atenção

- **Memória Redis ×4 nos writes** — mitigada pelos TTLs; all-time é o único que cresce sem bound (como hoje).
- **Mocks** de `LeaderboardStore`/`LeaderboardService` (service/transport tests) mudam assinatura de leitura — atualização mecânica.
- **BFF/painel não quebram**: params novos são opcionais; specs companheiras (`admin-bff/019`, `admin-ui/039`) só expõem o que já funciona por trás.
- Virada de período entre retry do SQS: a guarda de idempotência garante aplicação única; a janela é a do primeiro processamento (documentado na spec).
