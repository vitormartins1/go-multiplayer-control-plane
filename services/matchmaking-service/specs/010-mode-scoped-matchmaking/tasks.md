# Tasks: 010-mode-scoped-matchmaking

- [x] T1 — proto: `mode_id` nos 4 requests; `buf generate`
- [x] T2 — domain: Ports com `modeID` (Format/policies/Provision)
- [x] T3 — registryclient + profilecache: URL da coleção; cache por (game, mode)
- [x] T4 — matchmaker: ticket/tryForm/provision por (game, mode); sessionclient repassa `mode_id`
- [x] T5 — invader/summoner: política por modo; sinal com modeID; `ListSigns` filtra
- [x] T6 — transport: normalização `""→default` na borda + repasse
- [x] T7 — testes: casos da spec + migração dos mocks; suíte verde `-race`, lint
