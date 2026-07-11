# Spec: 008-orphan-reconcile

**Serviço:** session-service
**Status:** Done
**Data:** 2026-07-06
**ADR:** [0022](../../../../docs/adr/0022-orphan-session-reconcile-agones.md) — reconciliação de sessões órfãs contra o Agones

## Contexto

Uma sessão só fecha por `ReportResult`, `CloseByAdmin` ou o **TTL sweeper** (`OpenedAt + SESSION_TTL`, 30min). Quando um game server **abandona** a partida sem reportar resultado (crash, `Shutdown()` sem `ReportResult`, clients caem sob carga anormal, ou o timeout de alocação-sem-player do data plane), a sessão fica `OPEN`/"AO VIVO" no painel por até 30min sem gameserver por trás. A bateria de carga no Pi 5 (2026-07-05) deixou ~23 salas fantasma.

Esta feature faz o session-service **reconciliar** as sessões OPEN contra a fonte de verdade do pool (o Agones): a cada tick do sweeper, lista os `GameServer` vivos da fleet e fecha as sessões cujo server sumiu, com o motivo novo **`abandoned`**. O TTL sweeper permanece como backstop. Ver ADR-0022 para as alternativas descartadas (notificação no shutdown; só encurtar TTL).

## Comportamento

### Reconciliação (modo Agones)

- WHEN o tick do sweeper dispara E há um `ServerRegistry` configurado -> THE SYSTEM SHALL listar o conjunto de `GameServer` vivos da fleet e, para cada sessão OPEN cujo `ServerID` **não** está nesse conjunto E cujo `OpenedAt` é mais antigo que o **grace period**, fechá-la com `CloseReason = abandoned` (mesmo caminho unificado de `closeSession`: sai do mapa de ativas, entra no histórico, libera o server best-effort).
- THE SYSTEM SHALL respeitar um **grace period** (default 60s): sessões abertas há menos que isso **não** são reconciliadas nesse tick, para tolerar staleness/eventual-consistency do LIST e alocação recém-propagada.
- IF a listagem dos servers vivos falhar -> THE SYSTEM SHALL **pular a reconciliação nesse tick** (logar e seguir), sem fechar nenhuma sessão — uma falha de leitura no k8s não pode causar fechamento em massa de sessões saudáveis.
- THE SYSTEM SHALL contar as sessões fechadas por reconciliação sob `sessions_closed_total{reason="abandoned"}`, distinto de `ttl`/`admin`/`result`.

### TTL sweeper (inalterado, backstop)

- THE SYSTEM SHALL manter o `Sweep` por `OpenedAt + SESSION_TTL` como está — backstop de defesa em profundidade e único mecanismo automático no modo `static`.

### Modo static (dev, sem Agones)

- IF não há `ServerRegistry` configurado (modo `static` ou Agones sem registry) -> THE SYSTEM SHALL **não** reconciliar — cai só no TTL sweeper. A ausência de registry é o sinal de "sem fonte de verdade de liveness".

## Contratos de Interface

### Nova Port de saída (domain)

```go
// ServerRegistry é a Port de saída opcional que consulta a liveness do pool:
// quais game servers ainda existem. Só o adapter Agones implementa; o static
// não (seus servers nunca somem). Quando nil, o service não reconcilia.
type ServerRegistry interface {
    // LiveServerIDs devolve o conjunto de IDs de GameServers existentes na fleet.
    LiveServerIDs(ctx context.Context) (map[string]struct{}, error)
}
```

### Novo CloseReason (domain)

```go
// CloseAbandoned: fechada pela reconciliação — o game server sumiu da fleet
// sem ReportResult (ADR-0022).
CloseAbandoned CloseReason = "abandoned"
```

### Adapter Agones

- `LiveServerIDs` faz um `LIST` de `gameservers` no namespace, filtrado pelo label da fleet (`agones.dev/fleet=<fleet>`), e devolve o conjunto dos `metadata.name`. É o mesmo ID que `Allocate` grava em `Server.ID` (`status.gameServerName`).

### Wire-up (main)

- `buildAllocator` passa a devolver também um `domain.ServerRegistry` (o próprio adapter Agones, quando `ALLOCATOR_MODE=agones`; `nil` no `static`). O `Service` recebe o registry por `Option` (default `nil`).
- Novas envs: `SESSION_ORPHAN_GRACE` (default `60s`). A cadência de reconciliação **reusa** o `interval` do sweeper (1min) — não há env nova para isso.

## Fora de escopo

- Notificação de abandono pelo game server no shutdown (A2 do ADR-0022) — evolução complementar futura.
- TTL baseado em última atividade (A3) — mantido só `OpenedAt` como backstop.
- Watcher/informer do Agones (push em vez de poll) — o poll no tick do sweeper basta no volume atual; informer é evolução se a cadência doer.

## Critérios de aceite

- Sessão OPEN cujo `GameServer` some da fleet (após o grace period) fecha com `reason=abandoned` no próximo tick; o server é liberado; a métrica incrementa em `abandoned`.
- Sessão recém-criada (dentro do grace period) **não** é fechada mesmo que o LIST momentaneamente não a inclua.
- Falha no `LiveServerIDs` não fecha nenhuma sessão nesse tick.
- Modo `static` (sem registry) continua fechando só por TTL — comportamento inalterado.
- `go test ./... -race -cover` verde; cobertura do `service/` mantida ≥ 80%.
