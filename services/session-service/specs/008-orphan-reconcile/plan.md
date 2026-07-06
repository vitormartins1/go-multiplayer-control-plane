# Plan: 008-orphan-reconcile

**Spec:** [spec.md](./spec.md) · **ADR:** [0022](../../../../docs/adr/0022-orphan-session-reconcile-agones.md)

## Decisões técnicas

1. **Port opcional separada, não estender `ServerAllocator`.** `ServerRegistry` é uma interface nova em `domain/session.go`. Não a fundo em `ServerAllocator` porque o `static` não teria o que responder de forma honesta (seus servers nunca somem) e forçá-lo a implementar poluiria o adapter mock. O `Service` guarda um `registry domain.ServerRegistry` que pode ser `nil` — `nil` = "sem fonte de verdade de liveness, não reconcilie".

2. **Reconciliação no tick do sweeper, não numa goroutine nova.** `RunSweeper` já é o loop periódico com shutdown por context. Adiciono a reconciliação como um passo do mesmo tick (`reconcile(ctx, grace)` antes/depois do `Sweep`). Menos lifecycle para gerenciar, mesma cadência (1min) — coerente com o custo baixo do LIST.

3. **Grace period por `OpenedAt`.** Reuso `s.now()` (injetável, já usado pelo TTL) para testabilidade. Sessão só é candidata a `abandoned` se `OpenedAt.Before(now - grace)`. Protege contra falso-positivo por staleness do LIST logo após `Create`.

4. **Falha do LIST = pular o tick.** `LiveServerIDs` retornando erro → loga e retorna 0 fechadas, sem tocar em nenhuma sessão. Fechamento em massa por um blip de leitura no k8s seria pior que a órfã que estamos resolvendo.

5. **`CloseAbandoned` reusa `closeSession`.** O caminho unificado já faz delete do mapa + histórico + `closed[reason]++` + `Release` best-effort. Só passo o motivo novo. A métrica `sessions_closed_total{reason}` (CounterFunc no main) já lê `ClosedTotal(reason)` — preciso registrar o novo label lá.

6. **Adapter Agones: LIST com label selector.** `client.Resource(gameServerGVR).Namespace(ns).List(ctx, ListOptions{LabelSelector: "agones.dev/fleet="+fleet})`. Devolvo o conjunto de `metadata.name`. Consistente com `Allocate`, que usa `status.gameServerName` como `Server.ID`.

7. **`publicHost` decorator precisa propagar o registry.** O decorator embrulha o `ServerAllocator`; a `ServerRegistry` é do adapter concreto. `buildAllocator` devolve os dois separadamente (alloc decorado + registry cru do Agones), então o decorator não precisa saber de registry.

## Arquivos afetados

| Arquivo | Mudança |
|---------|---------|
| `internal/domain/session.go` | + interface `ServerRegistry`; + `CloseAbandoned` const |
| `internal/service/session_service.go` | + campo `registry` + `orphanGrace`; + `Option` `WithServerRegistry` e `WithOrphanGrace`; + método `reconcile(ctx, grace) int`; chamar no tick de `RunSweeper` |
| `internal/allocator/agones.go` | + método `LiveServerIDs` (LIST por label da fleet) na `*Agones` |
| `cmd/server/main.go` | `buildAllocator` devolve `(alloc, registry, err)`; passar `WithServerRegistry`/`WithOrphanGrace` ao `New`; env `SESSION_ORPHAN_GRACE`; registrar CounterFunc `abandoned` |
| `internal/service/session_service_test.go` | testes de `reconcile` (fecha órfã, respeita grace, erro no LIST, sem registry = no-op) |
| `internal/allocator/agones_test.go` | teste de `LiveServerIDs` (fake dynamic client) se já houver harness; senão, cobrir via fake do registry no service |

## Ordem de implementação

1. Domain: Port + const (base, sem dependências).
2. Service: campos, options, `reconcile`, hook no `RunSweeper`.
3. Testes do service (fake `ServerRegistry`) — TDD do core.
4. Adapter Agones: `LiveServerIDs`.
5. Wire-up no main + envs + métrica.
6. Rodar `go test ./... -race -cover`, `go build`, `golangci-lint`.
7. GO-LEARNINGS + observabilidade (novo label na série) + resumo didático.

## Riscos / pontos de atenção

- **Race no tick:** `reconcile` lê `s.sessions` sob `mu`, monta a lista de candidatos, solta o lock e fecha via `closeSession` (que re-trava). Mesmo padrão do `Sweep` — uma sessão fechada por `ReportResult` entre a leitura e o close devolve `ErrSessionNotFound`, ignorado. OK.
- **Fake do dynamic client** nos testes do adapter pode dar trabalho; se custar, cobrir a lógica de reconciliação inteira no `service` com um fake de `ServerRegistry` e deixar o adapter Agones com teste fino/manual (é I/O puro sobre o client k8s).
- **Cadência do LIST:** 1min basta hoje. Se a fleet crescer, migrar para informer (fora de escopo, anotado na spec).
