# Tasks: 031-p2p-topology

- [x] T1 — proto: campos `host_connection`/`world_token` (session) e `host_connection` (matchmaking); `buf generate`; `buf breaking` verde
- [x] T2 — session domain: `TopologyP2P`, `Session.HostConnection`/`WorldTokenHash`, `WorldEndpoint`, `ErrHostConnectionRequired`
- [x] T3 — session service: `OpenWorld` p2p sem allocator + world token; `JoinWorld` devolve `WorldEndpoint`; `ResolveWorldToken`; close/reconcile sem GameServer; testes
- [x] T4 — session transport: chamador `world_host` (`x-world-token`), checagem por mundo, respostas com `host_connection`; JSON admin com `topology`; testes
- [x] T5 — game-registry: `p2p` só em `host_world_dropin`; testes
- [x] T6 — matchmaking: repassar `host_connection` no `Invade` e no `SummonUpdate`; testes
- [x] T7 — admin-bff + admin-ui: `topology` na sessão; "P2P · host"; seletor libera p2p no `host_world_dropin`
- [x] T8 — demo p2p (`scripts/demo-host-world.sh` com `TOPOLOGY=p2p`) na bancada de binários
- [x] T9 — docs: CHANGELOG 1.4.0, contrato do arquétipo, ADR-0032, PROGRESSO/NEXT-STEPS, GO-LEARNINGS
