# Plan: 011-host-world-session

**Serviço:** session-service · **Spec:** `spec.md` (Aprovada 2026-07-07) · **ADR:** 0023 (arquétipo `host_world_dropin`, H0)

Como construir a sessão host-cêntrica. Decisões técnicas fechadas + arquivos a tocar. O `queue_arena` (specs 001–010) **não pode regredir** — `main` sempre verde.

## Decisões técnicas (as que a spec deixou em aberto)

### D1 — Identidade do mundo: rechavear `s.sessions` por `sessionID` + índice de `matchID`

Hoje `Service.sessions` é `map[matchID]*Session` e todo o caminho `queue_arena` indexa por `matchID`. O `host_world` **não tem matchID**. Decisão: **rechavear o mapa por `Session.ID`** (já um UUID único) e adicionar `matchIndex map[string]string` (matchID → sessionID) que **só o `queue_arena` popula**.

- `Create` (queue_arena): guarda `sessions[sess.ID] = sess` **e** `matchIndex[matchID] = sess.ID`.
- `OpenWorld` (host_world): guarda `sessions[sess.ID] = sess`; **não** toca `matchIndex`.
- `ReportResult(matchID)` / `CloseByAdmin(matchID)`: resolvem `sessionID := matchIndex[matchID]` antes de operar.
- `closeSession` passa a receber `sessionID` (não matchID); limpa `sessions[sessionID]` e, se houver, a entrada reversa em `matchIndex`.
- `Sweep`/`reconcile` já iteram os **valores** do mapa (pegam `sess`) — só o alvo do `delete` muda para `sess.ID`. Ganham `host_world` de graça.

**Por quê e não dois mapas separados:** um mapa único mantém sweeper, reconciliação, `history`, `AllSessions`/`SessionsByGame` e os counters funcionando para os dois arquétipos sem duplicar lógica. O custo é tocar o caminho `queue_arena` (indexação) — coberto pelos testes existentes de 001–010, que rodamos verdes antes de mergear. É a forma certa; dois mapas paralelos duplicariam os dois sweepers.

> **Risco assumido:** é o único ponto que mexe no caminho crítico do `queue_arena`. Mitigação: rodar `go test ./... -race` do session **antes** de qualquer código novo de host_world (baseline verde), e de novo depois.

### D2 — `ConnectTokenIssuer.Issue` ganha claims opcionais (não cresce a assinatura por arquétipo)

De `Issue(playerID, matchID, serverAddr, teamID string)` para:

```go
type ConnectClaims struct {
    TeamID string // queue_arena; omitido se vazio
    Role   string // host_world_dropin: "host"|"coop"|"invader"; omitido se vazio
}
Issue(playerID, refID, serverAddr string, claims ConnectClaims) (string, error)
```

- `refID` popula o claim `match_id` (mantido para o data plane do `queue_arena`). Para `host_world`, `refID = sessionID` — o token autoriza uma **sessão**, não um match. O contrato de claims do `host_world` (nome final: `match_id` genérico vs. `session_id` dedicado) é **formalizado em H2** (`host_world_dropin-dataplane-contract.md`); aqui basta emitir `role` corretamente.
- `role` entra como claim só quando não-vazio (mesmo padrão do `team_id`).
- Call site do `queue_arena` (`Create`) muda para `Issue(playerID, matchID, addr, ConnectClaims{TeamID: teamByPlayer[playerID]})` — mudança mecânica, um lugar.

### D3 — Nova Port de saída `ProfileProvider` no session (espelha o matchmaking)

O session hoje **não lê perfil**. Adiciono a Port + adapter HTTP ao game-registry, espelhando `matchmaking-service/internal/registryclient`:

```go
// domain
type WorldProfile struct {
    Archetype    string   // deve ser "host_world_dropin"
    EnabledRoles []string // "coop" e/ou "invader"
    CoopSlots    int
    InvaderSlots int
}
type ProfileProvider interface {
    WorldProfile(ctx context.Context, gameID string) (WorldProfile, error) // ErrProfileUnavailable
}
```

- Adapter novo `internal/registryclient/client.go`: `GET {GAME_REGISTRY_URL}/games/{id}/multiplayer-profile`, decodifica `archetype` + `params.{enabledRoles,coopSlots,invaderSlots}`. Arquétipo ≠ `host_world_dropin` → `ErrProfileUnavailable` (o service o mapeia para `ErrUnsupportedArchetype` no contexto do `OpenWorld`). Copiar o shape do adapter do matchmaking (mesmo endpoint, campos diferentes).
- **JSON confirmado** (`game-registry/internal/transport/http_handler.go` → `hostWorldDropInParamsDTO`): o endpoint serializa `archetype`, `topology`, `params.{enabledRoles, coopSlots (omitempty), invaderSlots (omitempty), levelRangeEnabled, weaponTierEnabled, passwordEnabled, invasionEnabled}`. O adapter do session lê `archetype` + `enabledRoles`/`coopSlots`/`invaderSlots`. Nenhuma divergência de tag.

### D4 — Alocação no `OpenWorld` (opção *a* do roadmap)

`OpenWorld` chama `allocator.Allocate(ctx, sessionID, gameID)` — reusa a Port existente. O `matchID` que o Agones grava como annotation passa a ser o `sessionID` no host_world (o adapter Agones não distingue; annotation é opaca). Alocação sob demanda (opção *b*) fica fora — evolução declarada.

### D5 — Sub-fatiamento

Manter **uma branch** `feat/session/011-host-world-session` com tudo. Ordem interna das tasks: domínio → issuer → adapter perfil → service (open/join/leave/close) → proto+transport → wire-up → observabilidade. Se o diff passar de ~600 linhas úteis, cortar em dois commits (H0a open/close+slots; H0b join/leave+role), **mas** mergear junto — a fatia só faz sentido completa (um mundo sem join não prova nada).

## Domínio (`internal/domain/session.go`, `errors.go`)

- `SessionOrigin` (`type` string) com `OriginMatch = "match"`, `OriginHostWorld = "host_world"`.
- `RoleSlot struct { Capacity int; Occupants []string }`; papéis `RoleHost/RoleCoop/RoleInvader` consts.
- `Session` ganha: `Origin SessionOrigin`, `HostPlayerID string`, `RoleSlots map[string]*RoleSlot`. `MatchID` fica vazio em host_world; `TeamByPlayer` nil.
- `MatchOutcome`/`WinLossDraw`/`PlayerOutcome` — **intocados** (resultado é H2).
- `SessionService` ganha `OpenWorld`, `JoinWorld`, `LeaveWorld`, `CloseWorld` (assinaturas da spec §"Ports").
- `ConnectTokenIssuer.Issue` — nova assinatura (D2) + `ConnectClaims`.
- Novo `ProfileProvider` + `WorldProfile` (D3).
- Novo `CloseReason` `CloseHostLeft = "host_left"`.
- Erros novos (`errors.go`): `ErrUnsupportedArchetype`, `ErrRoleNotEnabled`, `ErrRoleFull`, `ErrWorldNotFound`, `ErrAlreadyJoined`, `ErrProfileUnavailable`. Reusa `ErrNoServer`.

## Service (`internal/service/session_service.go`)

- `Service` ganha o campo `profiles domain.ProfileProvider` e `matchIndex map[string]string` (D1). `New` recebe o provider — decisão: adicionar como **parâmetro** (não Option), já que host_world é core agora; `queue_arena` passa o mesmo provider mas não o usa. *(Se preferir não mudar a assinatura de `New` e quebrar chamadas de teste, usar `WithProfileProvider` Option — decidir na task; recomendo parâmetro, é dependência de primeira classe.)*
- `OpenWorld(ctx, hostPlayerID, gameID)`:
  1. `profiles.WorldProfile(gameID)`; arquétipo ≠ host_world_dropin ou `ErrProfileUnavailable` → `ErrUnsupportedArchetype`.
  2. `allocator.Allocate(ctx, sessionID, gameID)` (gera `sessionID` via uuid antes); `ErrNoServer` propaga.
  3. Emite token do host: `issuer.Issue(hostPlayerID, sessionID, addr, ConnectClaims{Role: RoleHost})`. Falha → `allocator.Release` (rollback, como no `Create`).
  4. Monta `RoleSlots` a partir do perfil (só papéis em `EnabledRoles`, `Capacity` = CoopSlots/InvaderSlots). Registra a sessão (`Origin=host_world`, `OpenedAt=now()`), devolve `*Session` + `ConnectGrant` do host.
- `JoinWorld(ctx, sessionID, playerID, role)`: sob lock — acha o mundo (`ErrWorldNotFound` se ausente/closed); valida `role` em RoleSlots (`ErrRoleNotEnabled`); rejeita duplicado (`ErrAlreadyJoined`, varrendo occupants de todos os papéis); `ErrRoleFull` se cheio; ocupa o slot. **Fora do lock**, emite token (`ConnectClaims{Role: role}`) e devolve grant. *(Emitir token dentro do lock seria seguro mas prende o mutex numa op de cripto — emitir fora; se a emissão falhar, reverter o slot sob lock. Documentar essa ordem.)*
- `LeaveWorld(ctx, sessionID, playerID)`: remove o playerID do slot que o contém; **no-op idempotente** se não estiver em nenhum (retorna nil). Mundo segue aberto. Se `playerID == HostPlayerID`, delega para `CloseWorld`.
- `CloseWorld(ctx, sessionID)`: reusa `closeSession(sessionID, CloseHostLeft)` — libera o server, entra no history, encerra participação de todos (basta fechar a sessão; o data plane retorna os phantoms — cross-repo).
- `flattenTeams`, `stampTeams`, `validateOutcome`, `ReportResult`, `SessionsByGame`, `AllSessions` — inalterados exceto a rechaveação por sessionID (D1).

## Proto + transport (`proto/session/v1/session.proto`, `internal/transport/grpc_server.go`)

- Proto (aditivo): 4 RPCs novos no `service Session` + messages `OpenWorldRequest{host_player_id, game_id}`, `OpenWorldResponse{session_id, server_address, ConnectGrant host_grant}`, `JoinWorldRequest{session_id, player_id, role}`, `JoinWorldResponse{ConnectGrant grant}`, `LeaveWorldRequest{session_id, player_id}`, `LeaveWorldResponse{}`, `CloseWorldRequest{session_id}`, `CloseWorldResponse{}`. `ConnectGrant` reusado. `CreateSession`/`ReportResult` intocados.
- **Regenerar com `buf`** (gotcha do NEXT-STEPS): `PATH="$(go env GOPATH)/bin:$PATH" buf generate` dentro do serviço — nunca `sed` no `.pb.go`.
- Transport: 4 handlers novos, tradução pura + mapa de erros → gRPC codes (spec §"Erros"): `ErrUnsupportedArchetype`→`FailedPrecondition`, `ErrRoleNotEnabled`→`InvalidArgument`, `ErrRoleFull`/`ErrNoServer`→`ResourceExhausted`, `ErrWorldNotFound`→`NotFound`, `ErrAlreadyJoined`→`FailedPrecondition`, default→`Internal`.

## Wire-up (`cmd/server/main.go`)

- Montar o `ProfileProvider`: `http.Client` com `auth.InternalKeyTransport` + `otelhttp` (espelhar o `main.go` do matchmaking) e `GAME_REGISTRY_URL` (default `http://game-registry:8080`). Passar ao `service.New`.
- Registrar o counter `sessions_closed_total{reason="host_left"}` (mais um `NewCounterFunc`, como os 3 existentes).
- Novo gauge de slots por papel para o Rooms (H3) e o dashboard: `world_role_slots{role}` via `GaugeFunc` que soma occupants por papel entre os mundos abertos — expor `svc.OccupiedSlots(role) float64`. *(Ou adiar o gauge para H3; recomendo já expor — barato e o `game-lifecycle` ganha o funil host-world.)*
- Sweeper/reconcile/JWKS/healthz/metrics — inalterados (já cobrem host_world via D1).

## Observabilidade

- `sessions_closed_total{reason="host_left"}` cai no painel "Sessões encerradas/s por motivo" (label automático).
- `world_role_slots{role}` — novo gauge (acima).
- Tracing: os 4 RPCs entram no otelgrpc já instalado (StatsHandler global) — nada a fazer além dos handlers existirem.
- Checklist `docs/OBSERVABILITY.md`: sem novo banco (sessão é em memória), então nenhum exporter novo. RED coberto pelo gRPC.

## Testes

- **Baseline primeiro:** `go test ./services/session-service/... -race -cover` verde **antes** de codar (D1 é refactor de risco).
- `service`: `OpenWorld` (perfil ok → sessão+slots+token host; arquétipo errado → ErrUnsupportedArchetype; pool vazio → ErrNoServer; rollback do server se o token falhar). `JoinWorld` (slot livre; ErrRoleNotEnabled; ErrRoleFull; ErrWorldNotFound; ErrAlreadyJoined). `LeaveWorld` (libera slot; idempotente; host → fecha). `CloseWorld` (host_left, release). Mocks `testify/mock` para `ProfileProvider`/`ServerAllocator`/`ConnectTokenIssuer` (nunca struct real).
- `token`: `Issue` com `ConnectClaims{Role}` inclui claim `role`; `{TeamID}` mantém `team_id`; ambos vazios → nenhum extra. Não regride o queue_arena.
- `transport`: mapa de erros → codes dos 4 RPCs.
- `registryclient`: parse do perfil host_world; status ≠200/arquétipo errado → ErrProfileUnavailable (espelhar o teste do matchmaking).
- Cobertura ≥80% em `service/`/`domain/`.
- **Validação manual** (sem Unity): `grpcurl` OpenWorld→JoinWorld(coop)→JoinWorld(invader)→JoinWorld além da capacidade (ErrRoleFull)→LeaveWorld→CloseWorld; conferir slots no gauge e `host_left` no dashboard. Cliente de teste simula o phantom.

## Sequência de tasks (preview do `tasks.md`)

1. Baseline verde do session (`-race`).
2. Domínio: `SessionOrigin`, `RoleSlot`, campos no `Session`, novos erros, `CloseHostLeft`, Port `ProfileProvider`+`WorldProfile`, nova assinatura `Issue`+`ConnectClaims`, métodos na interface `SessionService`.
3. Token: `Issue` com `ConnectClaims` + testes; ajustar call site do `Create`.
4. Adapter `registryclient` (perfil host_world) + testes (JSON tags já confirmadas: `enabledRoles`/`coopSlots`/`invaderSlots`).
5. Service: D1 (rechaveação por sessionID + matchIndex) + `OpenWorld`/`JoinWorld`/`LeaveWorld`/`CloseWorld` + `OccupiedSlots` + testes.
6. Proto (4 RPCs) + `buf generate` + transport (handlers + erros) + testes.
7. Wire-up: `ProfileProvider` no `main`, counter `host_left`, gauge `world_role_slots`.
8. `go test ./... -race` verde (baseline + novo); validação `grpcurl`; observabilidade conferida.
9. GO-LEARNINGS: registrar o que for novo (ex.: tagged lifecycle de sessão com dois arquétipos no mesmo agregado; rechaveação sem regressão).

## Riscos / gotchas

- **D1 é o risco central** — rechavear o mapa toca o caminho `queue_arena`. Baseline verde antes/depois; os testes de 001–010 são a rede.
- **JSON tags do perfil** (D3): alinhar o adapter ao que o `game-registry/006` realmente serializa antes de codar (gotcha silencioso).
- **`buf generate`**, não `sed` no `.pb.go` (corrompe `rawDesc`).
- **Token fora do lock** no `JoinWorld` com reversão do slot no erro — não segurar o mutex durante cripto.
- Sem migration (sessão em memória) — nada de volume a recriar aqui.
