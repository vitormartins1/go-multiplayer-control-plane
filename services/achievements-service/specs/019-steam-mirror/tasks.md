# Tasks: 019-steam-mirror

- [x] T1 — identity 006: `GET /admin/players/{id}/external-identities` (repo, service, handler, testes)
- [x] T2 — migration 011 + `steamApiName` no domínio/repo/transport do catálogo (testes)
- [x] T3 — repo da fila: `ClaimSteamPending`, `MarkSteamSynced/Skipped/Retry/Failed` (integração)
- [x] T4 — adapters: Steam (`SetUserStatsForGame`) e identity client (httptest)
- [x] T5 — service `SteamMirror.SyncOnce` + backoff (mocks) e laço `Run`
- [x] T6 — consumer: envs, métrica, goroutine com lifecycle; compose
- [x] T7 — admin-bff/admin-ui: campo no formulário + selo no catálogo
- [x] T8 — validação ao vivo com Steam falsa; docs (README, PROGRESSO, guia Unity/pendências)
