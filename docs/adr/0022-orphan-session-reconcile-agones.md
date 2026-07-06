# ADR-0022: Reconciliação de sessões órfãs contra o Agones

**Status:** Accepted
**Data:** 2026-07-06
**Serviço(s) afetado(s):** session-service
**Relacionado:** ADR-0011 (Agones), ADR-0012 (allocator), ADR-0002 (hexagonal), ADR-0020 (fan-out durável)

---

## Contexto

Uma sessão do `session-service` só fecha por três caminhos: `ReportResult` (fim de partida reportado), `CloseByAdmin` (operador no console, spec 007) e o **TTL sweeper** (`Sweep` fecha o que está aberto há mais que `SESSION_TTL`, hoje 30min, medido a partir de `OpenedAt`).

Quando um game server **abandona** a partida sem reportar resultado — crash, `Shutdown()` sem `ReportResult`, clients caem antes do fim sob carga anormal, ou o novo *timeout de alocação-sem-player* do data plane — a sessão fica `OPEN`/"AO VIVO" no painel por **até 30min sem nenhum gameserver por trás**. A bateria de carga no Pi 5 (2026-07-05) deixou ~23 salas fantasma dessa forma.

Não é bug de métrica: `session_active_sessions` = `len(s.sessions)` e o `closeSession` faz `delete` corretamente. O problema é de **liveness**: nada fecha a sessão quando o server morre fora do fluxo feliz, e o `ServerAllocator` (Port de saída) só sabe `Allocate`/`Release` — **não sabe consultar** se um GameServer ainda existe.

No mundo Agones (ADR-0011/0012) o cluster **é** a fonte da verdade sobre "esse game server existe?": quando o GS morre, o Agones deleta/repõe o objeto `GameServer`. O session-service não está reconciliando contra isso.

## Alternativas Consideradas

### Como detectar o abandono

- **A1 — Reconciliar contra o Agones.** Uma nova Port de saída lista os `GameServer` vivos da fleet; toda sessão OPEN cujo `ServerID` sumiu é fechada. Cobre **qualquer** forma de abandono (inclusive crash), porque não depende do server cooperar. Custo: uma chamada LIST ao k8s por tick e acoplamento ao modelo de fleet (já acoplado via allocator).
- **A2 — Game server notifica abandono no `Shutdown`.** Novo RPC (ou `ReportResult` vazio com motivo). **Não cobre crash** (server morto não notifica) e exige mudar o data plane (repo `go-game-unity`, separado). Complementar, não substituto.
- **A3 — Só encurtar o TTL / basear em última atividade.** Paliativo: reduz a janela mas não a elimina, e confunde abandono com partida longa legítima (sem sinal de atividade, `OpenedAt` é tudo que há).

## Decisão

O session-service **reconcilia periodicamente** as sessões OPEN contra a fonte de verdade do pool (**A1**). Uma nova Port de saída `domain.ServerRegistry`:

```go
type ServerRegistry interface {
    // LiveServerIDs devolve o conjunto de GameServers existentes na fleet.
    LiveServerIDs(ctx context.Context) (map[string]struct{}, error)
}
```

é implementada **só pelo adapter Agones** (uma chamada LIST com selector de label da fleet — barata, um round-trip por tick, não N GETs). O `static` (pool mock de dev) **não** implementa: seus servers nunca somem, então a reconciliação é dispensável ali.

A reconciliação roda no **mesmo tick do sweeper**. Se há `ServerRegistry` configurado, a cada tick o service lista os servers vivos e fecha — com motivo novo **`abandoned`** — toda sessão OPEN cujo `ServerID` **não** está no conjunto, respeitando um **grace period** (só sessões abertas há mais que ~1min) para tolerar lag/eventual-consistency do LIST e alocação recém-propagada.

O **TTL sweeper permanece** como backstop (defesa em profundidade) e como o único mecanismo automático no modo `static`. `abandoned` é um `CloseReason` distinto de `ttl`/`admin`/`result`, então a métrica `sessions_closed_total{reason}` e o painel distinguem abandono de expiração.

Descartadas: **A2** (não cobre crash; cross-repo) — fica como evolução complementar futura se o data plane quiser antecipar o fechamento no shutdown limpo; **A3** (paliativo) — mantido apenas o que já existe (TTL) como backstop.

## Consequências

### Positivas

- Órfãs fecham em **segundos** (próximo tick), não em até 30min; game server liberado ao pool mais cedo.
- Cobre **todas** as formas de abandono, inclusive crash — não depende do server cooperar.
- Métrica/painel distinguem `abandoned` de `ttl` (observabilidade do fenômeno).
- Hexagonal preservado: Port opcional separada; `static` intocado; core continua só dependendo de interfaces do `domain`.

### Negativas / Trade-offs

- Uma chamada LIST ao k8s API por tick do sweeper (barata no volume de dev; reavaliar cadência se a fleet crescer muito).
- Acopla o session-service ao modelo de fleet do Agones — mas o allocator já está acoplado (ADR-0011).
- O grace period introduz uma janela mínima antes de fechar (aceito: protege contra falso-positivo por staleness do LIST).

### Neutras / Implicações

- Observabilidade: novo valor `abandoned` na série `sessions_closed_total{reason}`; candidato a card no dashboard de lifecycle.
- Spec: `session-service/008-orphan-reconcile`.
- A2 (notificação no shutdown) continua no backlog como complemento, não substituto.

## Referências

- ADR-0011 — Agones como orquestrador de game servers (a fonte da verdade da liveness).
- ADR-0012 — allocator via GameServerAllocation.
- NEXT-STEPS backlog item 12 — achado da bateria de carga no Pi 5 (2026-07-05).
