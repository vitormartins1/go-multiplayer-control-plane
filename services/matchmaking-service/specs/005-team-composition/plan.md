# Plan: 005-team-composition

**Spec:** `spec.md` (Aprovada) · **ADR:** 0023 · **Depende de:** `session/010` (contrato de `Provision`/`CreateSession`)

## Decisões técnicas

### D1 — `ProfileProvider` devolve a formação, não só o tamanho
Substitui `MatchSize(ctx, gameID) (int, error)` por `Format(ctx, gameID) (MatchFormat, error)`:
```
MatchFormat{ Size int; Structure string; TeamCount int; TeamSize int }
```
`Structure` ∈ {`"ffa"`,`"teams"`}. `Size` continua sendo o critério de agrupamento (ffa: playerCount; teams: teamCount×teamSize). A derivação segue no adapter `registryclient` (já lê os 4 campos).

### D2 — Atribuição posicional determinística (blocos contíguos)
Grupo FIFO de `Size` tickets. `teams`: time `k` (0-based) = `players[k*teamSize : (k+1)*teamSize]`, rótulo `team-<k+1>`. `ffa`: um único assignment com `TeamID:""` e todos os players. Determinístico e testável; **sem skill/MMR** (backlog).

### D3 — `SessionProvisioner.Provision` leva a composição
`Provision(ctx, matchID string, teams []TeamAssignment, gameID string)` com `TeamAssignment{ TeamID string; Players []string }` (domínio do matchmaking). O adapter `sessionclient` monta `CreateSessionRequest{ players: união, teams: assignments (omitidos em ffa) }`.

### D4 — Ticket e cache carregam a formação
`ticket` guarda `format MatchFormat` (não só `matchSize`). `profilecache` cacheia `MatchFormat` por gameID (TTL atual). `tryForm` agrupa por `format.Size` (igual hoje) e particiona conforme `format.Structure`.

## Arquivos

| Arquivo | Mudança |
|---|---|
| `internal/domain/matchmaking.go` | `MatchFormat`; `TeamAssignment`; `ProfileProvider.Format`; `SessionProvisioner.Provision` assinatura |
| `internal/registryclient/client.go` | implementa `Format` (devolve structure/teamCount/teamSize + size derivado) |
| `internal/profilecache/cache.go` | cacheia `MatchFormat` |
| `internal/service/matchmaker.go` | `ticket.format`; `Join` usa `Format`; `tryForm`/`provision` particionam e passam `teams` |
| `internal/sessionclient/grpc_client.go` | `Provision` monta `players`+`teams` no request |
| testes | particionamento (2×1, 2×2, 3×3, ffa); Join com Format; provision leva teams |

## Ordem
domain → registryclient → profilecache → matchmaker → sessionclient → testes → build/`-race`/lint. (Depois do `session/010`, que define o proto/contrato.)
