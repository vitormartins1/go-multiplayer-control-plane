# Tasks 040 — UX da página do jogador

## identity-service (spec identity/004)

- [x] T1: `FindByID` no `AccountRepository` (domain + postgres, projeção sem hash)
- [x] T2: `GetPlayer` no `PlayerLister` (domain + service, erro wrappado)
- [x] T3: rota `GET /admin/players/{playerID}` (200 / 404 `player_not_found`) + testes

## admin-bff (spec admin-bff/020)

- [x] T4: `GetPlayer` na Port `PlayerDirectory` + `ErrPlayerNotFound` (domain)
- [x] T5: `GetPlayer` no client identity (404 → not-found; resto → unavailable)
- [x] T6: `Player` no service + rota `GET /api/players/{playerID}` (200/404/502) + testes

## admin-ui

- [x] T7: `api.player(playerID)` em `lib/api.ts`
- [x] T8: header/breadcrumb com handle (guest → "Convidado" + badge; falha → fallback)
- [x] T9: `GameChipsSelector` (chips com mini-capa, ≤5 visíveis, "+N" com Menu,
      selecionado sempre visível) substituindo o `Select` no `PlayerDetail`
- [x] T10: `fromGameId` via router state (`GamePlayers` → `PlayerDetail`)
- [x] T11: barras de progresso por linha (ambas as visões) mantendo numeral
- [x] T12: `SegmentedControl` + visão "Por estado" (3 tabelas com contagem/vazio)
- [x] T13: progresso geral ponderado (barra + "X de N · Y%" + tooltip)

## Fechamento

- [x] T14: `go test ./... -race` (identity + admin-bff), `golangci-lint`, `npm run build`
- [x] T15: smoke manual no compose (casos da spec) + GO-LEARNINGS + resumo didático
