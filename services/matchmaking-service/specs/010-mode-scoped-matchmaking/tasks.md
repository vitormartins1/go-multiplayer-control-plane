# Tasks: 010-mode-scoped-matchmaking

- [ ] T1 — proto: `mode_id` nos 4 requests; `buf generate`
- [ ] T2 — domain: Ports com `modeID` (Format/policies/Provision)
- [ ] T3 — registryclient + profilecache: URL da coleção; cache por (game, mode)
- [ ] T4 — matchmaker: ticket/tryForm/provision por (game, mode); sessionclient repassa `mode_id`
- [ ] T5 — invader/summoner: política por modo; sinal com modeID; `ListSigns` filtra
- [ ] T6 — transport: normalização `""→default` na borda + repasse
- [ ] T7 — testes: casos da spec + migração dos mocks; suíte verde `-race`, lint
