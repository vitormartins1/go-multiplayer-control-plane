# Tasks: 017-mode-snapshot-manifest

- [x] T1 — proto: `mode_id` (Create/OpenWorld), RPC `GetSessionConfig`; `buf generate`
- [x] T2 — domain: `ProfileSnapshot` (substitui `WorldProfile`), `SessionManifest`, `Session.ModeID/Profile`, constantes de arquétipo, `ErrOutcomeMismatch`, Ports novas/alteradas
- [x] T3 — registryclient: `ModeProfile` (URL da coleção, params crus, hash); profilecache decorator
- [x] T4 — service: snapshot obrigatório no `Create`/`OpenWorld`, `GetSessionConfig`, validação kind×arquétipo no `ReportResult`
- [x] T5 — token: claim `mode_id` sempre presente (Create/OpenWorld/JoinWorld)
- [x] T6 — publishers: `modeId` nos 4 envelopes (schemaVersion 2 mantido)
- [x] T7 — transport: gRPC (normalização default, GetSessionConfig, FailedPrecondition) + `modeId` no board admin (e passthrough tipado no admin-bff se houver)
- [x] T8 — main: wiring do cache
- [x] T9 — testes: casos da spec + migração mecânica dos mocks/arranges; suíte verde `-race`, lint
- [ ] T10 — docs: 4 contratos de data plane (claim `mode_id` + seção manifest + migração first-to-5 no queue_arena)
