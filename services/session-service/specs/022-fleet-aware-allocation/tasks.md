# Tasks 022 — allocator fleet-aware

- [ ] T1. `domain.ProfileSnapshot.Fleet` + `domain.ErrFleetUnavailable` + `ServerAllocator.Allocate` com parâmetro `fleet`
- [ ] T2. Adapter Agones: remove `fleet` do construtor/struct, recusa vazio, selector usa o parâmetro
- [ ] T3. Adapter static: assinatura nova, `fleet` ignorado
- [ ] T4. `registryclient`: `modeResponse.Fleet` + `snap.Fleet`
- [ ] T5. `session_service.go`: `Create`/`OpenWorld` repassam `snap.Fleet` para `Allocate`
- [ ] T6. gRPC: `ErrFleetUnavailable` → `FailedPrecondition` em `CreateSession` e `worldError`
- [ ] T7. `main.go`: remove `AGONES_FLEET`/parâmetro do `NewAgones`
- [ ] T8. Testes (allocator ×2, registryclient, service, transport) + suite completa `-race -cover`
- [ ] T9. (Rollout, não-código) Anotar no runbook/README do session-service o passo manual: setar `fleet` do jogo Agones-real antes do deploy
