# Plan: 012-invadable-world-directory

**Serviço:** session-service · **Spec:** `spec.md` (Aprovada 2026-07-07) · **ADR:** 0023 (H1a, lado session)

Como o session passa a (a) guardar os atributos de matchmaking do host no `OpenWorld` e (b) expor `ListInvadableWorlds`. Fatia **aditiva** — H0 (spec 011) e queue_arena não regridem.

## Decisões técnicas

### D1 — atributos do host via struct `WorldAttributes` no `OpenWorld`

`OpenWorld(ctx, hostPlayerID, gameID string)` passa a `OpenWorld(ctx, hostPlayerID, gameID string, attrs domain.WorldAttributes)` com `WorldAttributes{SoulLevel, WeaponTier int}`. Um struct (não dois ints posicionais) para não crescer a assinatura a cada evolução (senha/região virão). `Session` ganha `HostSoulLevel int` e `WeaponTier int`, preenchidos no `OpenWorld`. Zeros = mundo sem faixa (queue_arena e host sem atributos).

- Call site do transport (`grpc_server.go` → `OpenWorld` handler) passa `domain.WorldAttributes{SoulLevel: int(req.GetHostSoulLevel()), WeaponTier: int(req.GetWeaponTier())}`.
- Nenhum outro chamador de `OpenWorld` existe (o `main` não chama).

### D2 — `ListInvadableWorlds` varre o mapa sob o lock (como `SessionsByGame`)

Novo método no `Service`:

```go
func (s *Service) ListInvadableWorlds(ctx context.Context, gameID string) ([]domain.InvadableWorld, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    var out []domain.InvadableWorld
    for _, sess := range s.sessions { // só ativas — closeSession já removeu as fechadas
        if sess.Origin != domain.OriginHostWorld || sess.GameID != gameID { continue }
        slot, ok := sess.RoleSlots[domain.RoleInvader]
        if !ok { continue } // papel invasor não habilitado
        free := slot.Capacity - len(slot.Occupants)
        if free <= 0 { continue } // cheio
        out = append(out, domain.InvadableWorld{
            SessionID: sess.ID, HostSoulLevel: sess.HostSoulLevel,
            WeaponTier: sess.WeaponTier, FreeInvaderSlots: free,
        })
    }
    return out, nil
}
```

- **State check implícito:** o mapa `s.sessions` só contém sessões OPEN (`closeSession` as remove) — não precisa checar `State`.
- **Sem ordenação garantida** nesta fatia (a iteração de mapa em Go é aleatória). O matchmaking (`006`) não depende de ordem — ele filtra e tenta em ordem de retorno. *(Se quisermos estabilidade depois, ordena por `OpenedAt`; não agora.)*
- **Fonte de dados pura:** nenhum filtro de faixa/weapon/senha aqui (política é do matchmaking). Devolve `error` na assinatura por consistência com as outras Ports, mas na prática nunca erra (vazio = lista vazia).

### D3 — proto aditivo + `buf`

- `OpenWorldRequest` += `int32 host_soul_level = 3; int32 weapon_tier = 4;` (campos 3/4, aditivos).
- Novo RPC `ListInvadableWorlds` + messages `ListInvadableWorldsRequest{string game_id}`, `ListInvadableWorldsResponse{repeated InvadableWorld worlds}`, `InvadableWorld{string session_id; int32 host_soul_level; int32 weapon_tier; int32 free_invader_slots}`.
- **`JoinWorldResponse` += `string server_address`** (aditivo). Furo de contrato herdado da H0: o `JoinWorldResponse` devolvia só o `ConnectGrant` (token), mas o phantom precisa do **endereço do server** para conectar — o `matchmaking/006` não teria o que pôr no `server_address` do `Invade`. O `Session` já tem `ServerAddress`; o handler `JoinWorld` passa a devolvê-lo. Toca também o service (`JoinWorld` retorna o addr junto do grant) — mudança pequena, mas mexe na assinatura interna do `JoinWorld`. *(Justifica-se aqui, não numa fatia à parte: sem o addr, a injeção da H1a não é utilizável.)*
- `PATH="$(go env GOPATH)/bin:$PATH" buf generate` dentro do serviço (nunca `sed` — gotcha do `rawDesc`).

### D4 — transport

- Handler `OpenWorld`: passa `WorldAttributes` (D1).
- Novo handler `ListInvadableWorlds`: traduz o request, chama `svc.ListInvadableWorlds`, mapeia `[]domain.InvadableWorld` → `[]*pb.InvadableWorld`. Sem erro de domínio novo → só `Internal` no default.

### D5 — observabilidade adiada para H3

O gauge `invadable_worlds{game}` fica para a H3 (surfacing no console). Motivo: gauge com label por jogo pede gestão de cardinalidade, e o funil de invasão só ganha sentido visual junto com o `invasions_total` do matchmaking (`006`) e a aba Rooms. Registrado como pendência da H3, não desta fatia. Tracing dos 2 RPCs já vem do otelgrpc global.

## Domínio (`internal/domain/session.go`)

- `Session` += `HostSoulLevel int`, `WeaponTier int`.
- Novo `WorldAttributes{SoulLevel, WeaponTier int}`.
- Novo `InvadableWorld{SessionID string; HostSoulLevel, WeaponTier, FreeInvaderSlots int}`.
- `SessionService.OpenWorld` nova assinatura + `ListInvadableWorlds(ctx, gameID) ([]InvadableWorld, error)`.

## Arquivos a tocar

| Arquivo | Mudança |
|---|---|
| `internal/domain/session.go` | campos no `Session`, `WorldAttributes`, `InvadableWorld`, assinaturas na Port |
| `internal/service/session_service.go` | `OpenWorld` recebe/guarda attrs; novo `ListInvadableWorlds` |
| `internal/service/host_world_test.go` | testes de attrs + diretório |
| `proto/session/v1/session.proto` | campos + RPC + messages; `buf generate` |
| `internal/transport/grpc_server.go` | `OpenWorld` passa attrs; handler `ListInvadableWorlds` |
| `internal/transport/host_world_grpc_test.go` | e2e do diretório via bufconn |

`cmd/server/main.go` — **sem mudança** (não há dependência nova; `OpenWorld` não é chamado no wire-up).

## Testes

- **Baseline** `go test ./services/session-service/... -race` verde antes.
- `service`: `OpenWorld` guarda soul level + weapon tier; sem attrs abre com zeros. `ListInvadableWorlds`: só mundos abertos do jogo com invasor livre; exclui invasor cheio, sem papel invasor, outro jogo, `queue_arena`; `freeInvaderSlots` correto; nenhum elegível → lista vazia (nil/len 0), sem erro. Reusa `openWorldSetup` (ajustar para a nova assinatura de `OpenWorld`).
- `transport` (bufconn): OpenWorld com attrs → ListInvadableWorlds reflete; após encher o slot de invasor, o mundo some da lista.
- Cobertura ≥80% em `service/`/`domain/`.

## Sequência de tasks (preview)

1. Baseline verde.
2. Domínio: campos + `WorldAttributes` + `InvadableWorld` + assinaturas (inclui `JoinWorld` devolvendo `server_address`).
3. Service: `OpenWorld` guarda attrs; `JoinWorld` retorna o `ServerAddress` junto do grant; `ListInvadableWorlds` + testes.
4. Proto (campos + RPC + `server_address` no `JoinWorldResponse`) + `buf generate` + transport (handlers) + testes bufconn.
5. `go test ./... -race` verde (baseline + novo); lint.
6. Nada de GO-LEARNINGS provável (é aditivo, mesmo padrão de varredura sob lock do H0/spec 005) — registrar só se surgir algo.

## Riscos / gotchas

- **Assinatura de `OpenWorld`** muda — ajustar o call site do transport e o helper `openWorldSetup` dos testes da H0. É o único ripple (poucos lugares).
- **`buf generate`**, nunca `sed`.
- Sem migration (sessão em memória).
