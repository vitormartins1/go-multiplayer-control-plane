# Tasks: 030-grpc-auth

- [x] T1 proto `summoner_player_id` + buf generate
- [x] T2 service: `ErrNotWorldHost`, `JoinWorld` com summoner, `IsWorldHost` + testes
- [x] T3 transport: interceptor (tabela × modos) + chamador no ctx + testes
- [x] T4 transport: `OpenWorld`/`LeaveWorld`/`CloseWorld` usam o chamador; mapeamento de erros + testes
- [x] T5 main: wiring (verifier, chaves, modo, métrica)
- [x] T6 matchmaking: credenciais por RPC, summoner no `Inject`, `PermissionDenied` + testes
- [x] T7 infra: compose, fleets, demo em `enforce` (validado ao vivo)
- [x] T8 docs: contratos, CHANGELOG 1.2.0, GO-LEARNINGS
