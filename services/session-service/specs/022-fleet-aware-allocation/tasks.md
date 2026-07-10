# Tasks 022 — allocator fleet-aware

- [x] T1. `domain.ProfileSnapshot.Fleet` + `domain.ErrFleetUnavailable` + `ServerAllocator.Allocate` com parâmetro `fleet`
- [x] T2. Adapter Agones: remove `fleet` do construtor/struct, recusa vazio, selector usa o parâmetro
- [x] T3. Adapter static: assinatura nova, `fleet` ignorado
- [x] T4. `registryclient`: `modeResponse.Fleet` + `snap.Fleet`
- [x] T5. `session_service.go`: `Create`/`OpenWorld` repassam `snap.Fleet` para `Allocate`
- [x] T6. gRPC: `ErrFleetUnavailable` → `FailedPrecondition` em `CreateSession` e `worldError`
- [x] T7. `main.go`: remove `AGONES_FLEET`/parâmetro do `NewAgones`
- [x] T8. Testes (allocator ×2, registryclient, service, transport) + suite completa `-race -cover` (service 94.1%, transport 88.0%, registryclient 84.8%, allocator 85.2%)
- [x] T9. (Rollout, não-código) Runbook anotado no `README.md` do session-service: setar `fleet` do jogo antes de habilitar `ALLOCATOR_MODE=agones`

## Achado fora do escopo original, corrigido na mesma fatia

`LiveServerIDs` (reconciliação, ADR-0022) filtrava `GameServer`s por uma Fleet
fixa — com múltiplas Fleets por jogo agora possíveis (o próprio propósito do
ADR-0026), o sweeper deixaria de reconciliar sessões dos jogos que não fossem
a única fleet configurada. Corrigido: lista o namespace inteiro, sem filtro de
label (`internal/allocator/agones.go`).
