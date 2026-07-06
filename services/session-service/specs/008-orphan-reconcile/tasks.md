# Tasks: 008-orphan-reconcile

**Spec:** [spec.md](./spec.md) · **Plan:** [plan.md](./plan.md)

Branch: `feat/session/008-orphan-reconcile`

## Domain

- [x] T1 — `domain/session.go`: adicionar interface `ServerRegistry` com `LiveServerIDs(ctx) (map[string]struct{}, error)` + doc comentando que só o Agones implementa.
- [x] T2 — `domain/session.go`: adicionar const `CloseAbandoned CloseReason = "abandoned"` com comentário (ADR-0022).

## Service (core)

- [x] T3 — `service`: adicionar campos `registry domain.ServerRegistry` e `orphanGrace time.Duration` (default 60s) no `Service`.
- [x] T4 — `service`: `Option` `WithServerRegistry(domain.ServerRegistry)` e `WithOrphanGrace(time.Duration)`.
- [x] T5 — `service`: método `reconcile(ctx, grace) int` — lista vivos (skip se `registry == nil`); se `LiveServerIDs` erra, loga e retorna 0; fecha (`CloseAbandoned`) as OPEN cujo `ServerID` não está no conjunto E `OpenedAt` antes de `now-grace`.
- [x] T6 — `service`: chamar `reconcile` no tick do `RunSweeper` (junto do `Sweep`), com log quando fechar > 0.

## Testes do service (TDD)

- [x] T7 — fake `ServerRegistry` no `_test.go`.
- [x] T8 — `TestReconcile_ServerSumiuAposGrace_FechaComoAbandoned`.
- [x] T9 — `TestReconcile_DentroDoGrace_NaoFecha`.
- [x] T10 — `TestReconcile_ErroNoList_NaoFechaNada`.
- [x] T11 — `TestReconcile_SemRegistry_NoOp`.
- [x] T12 — verificar `ClosedTotal(CloseAbandoned)` incrementa e server é liberado (fake allocator conta Release).

## Adapter Agones

- [x] T13 — `allocator/agones.go`: método `LiveServerIDs` (LIST `gameservers` por label `agones.dev/fleet=<fleet>`, devolve set de `metadata.name`). `var _ domain.ServerRegistry = (*Agones)(nil)`.

## Wire-up + observabilidade

- [x] T14 — `main.go`: `buildAllocator` devolve `(alloc, registry, err)` — registry = adapter Agones no modo agones, `nil` no static.
- [x] T15 — `main.go`: env `SESSION_ORPHAN_GRACE` (default 60s); passar `WithServerRegistry`/`WithOrphanGrace` no `New`.
- [x] T16 — `main.go`: registrar CounterFunc `sessions_closed_total{reason="abandoned"}` (ler `ClosedTotal(domain.CloseAbandoned)`).

## Fechamento

- [x] T17 — `go test ./... -race -cover` (≥80% no service), `go build ./cmd/server/...`, `golangci-lint run`.
- [x] T18 — GO-LEARNINGS: se surgir padrão novo (Port opcional via nil + type-safety, reconcile loop) registrar; senão, dizer que não houve.
- [x] T19 — Observabilidade: anotar o novo label `abandoned` na série `sessions_closed_total` (dashboard de lifecycle, se couber).
- [x] T20 — Commit convencional + merge `--no-ff` na `main` (main verde).
