# Topologia de repositórios — control plane + data planes por arquétipo

> **Status:** **nomes decididos (2026-07-07); rename ainda não executado.** O padrão abaixo é a fonte de verdade da nomenclatura — os repos atuais seguem com os nomes antigos até uma janela dedicada de rename; repos novos já nascem com o nome do padrão.
>
> **Motivação:** o ADR-0023 generalizou o multiplayer por **arquétipo** (`queue_arena`, `battle_royale`, `coop_mission`, `host_world_dropin`). A tese de validação (`docs/game-models/`) é ter **um data plane de referência por arquétipo** — cada um é um projeto Unity próprio, com um repo próprio, para testar e validar aquele tipo de jogo de forma isolada. Com N data planes chegando, o nome genérico atual (`go-game-dataplane`) deixa de escalar.

## Princípios

1. **Um control plane, N data planes** (ADR-0010). O control plane (Go) é único e agnóstico de arquétipo — orienta-se pelo perfil (ADR-0023). Cada arquétipo tem seu **data plane** (Unity) de referência, em repo separado.
2. **O data plane é do arquétipo, não do jogo.** O repo representa a *forma* de multiplayer (`queue_arena`), não um jogo/tema específico (câmera, arte, economia são conteúdo — ficha 05: gênero ≠ arquétipo). Por isso o nome do repo usa o **vocabulário do arquétipo do ADR-0023**, não um nome de produto.
3. **Forma certa acima de compatibilidade do data plane.** Ao evoluir o backend, priorizamos o design correto do control plane mesmo que isso quebre um data plane já implementado. O data plane migra depois; as mudanças cross-repo ficam documentadas (ex.: `docs/game-models/queue_arena-dataplane-contract.md`). Confirmado pelo dono do projeto em 2026-07-06.

## Padrão de nomenclatura (decidido)

```
go-multiplayer-control-plane        # o backend Go (único)
unity-<archetype>-data-plane        # um por arquétipo (Unity)
```

- **Control plane:** `go-multiplayer-control-plane` — prefixo `go-` (a linguagem), "multiplayer" (o domínio da tese), "control-plane" (o papel, ADR-0010).
- **Data planes:** prefixo `unity-` (o engine), `<archetype>` = o enum do arquétipo (ADR-0023) em **kebab-case** (`queue-arena`, `battle-royale`, `coop-mission`, `host-world-dropin`), sufixo `-data-plane`. A simetria `control-plane` ↔ `data-plane` é o eixo organizador.

## Mapa de repositórios

| Arquétipo (ADR-0023) | Repo data plane (nome decidido) | Repo atual | Status |
|---|---|---|---|
| `queue_arena` | `unity-queue-arena-data-plane` | `go-game-dataplane` | ✅ existe (referência; jogo "Packet Arena") — **renomear** |
| `battle_royale` | `unity-battle-royale-data-plane` | — | ⬜ planejado (criar quando atacar o arquétipo) |
| `coop_mission` | `unity-coop-mission-data-plane` | — | ⬜ planejado |
| `host_world_dropin` | `unity-host-world-dropin-data-plane` | — | ⬜ planejado |
| — (control plane) | `go-multiplayer-control-plane` | `go-game-backend` | ✅ existe — **renomear** |

> `go-game-unity` (repo vazio, órfão): resquício do plano antigo (memória "dataplane-repo"). Data plane real virou `go-game-dataplane`. **Arquivar/remover** no rename.

```mermaid
flowchart TB
  CP["go-multiplayer-control-plane (Go)<br/>identity · matchmaking · session · leaderboard · ...<br/>orientado por perfil (ADR-0023)"]
  QA["unity-queue-arena-data-plane<br/>(Unity) ✅ existe"]
  BR["unity-battle-royale-data-plane<br/>⬜ planejado"]
  CM["unity-coop-mission-data-plane<br/>⬜ planejado"]
  HW["unity-host-world-dropin-data-plane<br/>⬜ planejado"]
  CP -- "perfil queue_arena" --> QA
  CP -- "perfil battle_royale" --> BR
  CP -- "perfil coop_mission" --> CM
  CP -- "perfil host_world_dropin" --> HW
```

## Ao executar o rename (checklist futuro — não fazer agora)

Renomear toca o module path Go e várias referências. Quando decidido:

1. **Repos GitHub:** renomear (GitHub redireciona o antigo por um tempo) e `git remote set-url`.
2. **Module path Go** (`go-game-backend` → `go-multiplayer-control-plane`): `go.mod` + todos os imports `github.com/vitormartins1/go-game-backend/...` (ADR-0001, módulo único). Impacta o stub gerado que o data plane consome (proto go_package).
3. **Referências em docs do backend** (ajustar os links relativos `../go-game-dataplane` e menções): `docs/ARCHITECTURE.md`, `docs/NEXT-STEPS.md`, `docs/ROADMAP.md`, `docs/UNITY-INTEGRATION.md`, `docs/game-models/*`, ADRs 0010/0011/0012/0020/0021/0022/0023 e `docs/adr/README.md`.
4. **Referências no data plane** (`go-game-dataplane`): README, CLAUDE.md, ROADMAP, specs, links `../go-game-backend`.
5. **Memórias** do assistente que citam nomes de repo (`dataplane-repo`, etc.).
6. **CI/CD e paths** que embutam o nome do repo/módulo.

> Sugestão de sequência: renomear os repos → module path → varredura de referências → validar build/CI. Feito de uma vez para não conviver com nomes mistos.

## Próximos passos

- [x] ~~Fechar os nomes~~ — decididos em 2026-07-07 (control plane `go-multiplayer-control-plane`; data planes `unity-<archetype>-data-plane`).
- [ ] Criar os repos data plane planejados conforme cada arquétipo for atacado (já com o nome do padrão).
- [ ] Executar o rename (checklist) numa janela dedicada.
- [ ] (Opcional) Registrar como **ADR** se quiser tornar a convenção formal — hoje é decisão deste doc.
