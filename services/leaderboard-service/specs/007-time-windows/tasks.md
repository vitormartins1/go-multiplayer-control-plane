# Tasks: 007-time-windows

- [ ] T1 — domain: consts `Window*` + `ValidWindow` + `ErrInvalidWindow`; Ports (`Store.Rank/Top/Range` e `Service.Top/Around` com `window`)
- [ ] T2 — storage: `windowKey` (UTC, ISOWeek), `now` injetável, TTL consts; `Submit`/`ForceScore` via pipeline nas 4 janelas + EXPIRE
- [ ] T3 — storage: `ApplyCumulative` expande incrs ×4 janelas no script (EXPIRE atômico junto); guarda única
- [ ] T4 — service: validação de window (default alltime), clamp de offset, repasse; `Submit` devolve entry do all-time
- [ ] T5 — transport: `?window` nos 3 GETs, `?offset` nos 2 tops, `ErrInvalidWindow` → 400 `invalid_window`
- [ ] T6 — testes service: window inválida/default, offset clamp, mocks atualizados
- [ ] T7 — testes storage (integração/testcontainers, relógio fixado): write espalha 4 keys; leitura por janela; TTL nas periódicas (ausente na all-time); dup do cumulativo é no-op em todas; paginação com ranks contínuos; virada de período (relógio avança → key nova)
- [ ] T8 — testes transport: 400 window inválida; propagação window/offset; respostas sem params novos idênticas às atuais
- [ ] T9 — barra verde: build, `go test ./... -race`, integração storage, lint 0
- [ ] T10 — docs: spec `Status: Aprovado`; `docs/NEXT-STEPS.md` (backlog item 6 → parcial: janelas ✅, seasons nomeadas adiadas)
