# Topologia de repositórios — control plane + data planes por arquétipo

> **Status:** proposta / decisão pendente (documentado 2026-07-06). **Nada é renomeado ainda** — este doc fixa o padrão para quando executarmos o rename e para criar os próximos repos já com o nome certo.
>
> **Motivação:** o ADR-0023 generalizou o multiplayer por **arquétipo** (`queue_arena`, `battle_royale`, `coop_mission`, `host_world_dropin`). A tese de validação (`docs/game-models/`) é ter **um data plane de referência por arquétipo** — cada um é um projeto Unity próprio, com um repo próprio, para testar e validar aquele tipo de jogo de forma isolada. Com N data planes chegando, o nome genérico atual (`go-game-dataplane`) deixa de escalar.

## Princípios

1. **Um control plane, N data planes** (ADR-0010). O control plane (Go) é único e agnóstico de arquétipo — orienta-se pelo perfil (ADR-0023). Cada arquétipo tem seu **data plane** (Unity) de referência, em repo separado.
2. **O data plane é do arquétipo, não do jogo.** O repo representa a *forma* de multiplayer (`queue_arena`), não um jogo/tema específico (câmera, arte, economia são conteúdo — ficha 05: gênero ≠ arquétipo). Por isso o nome do repo usa o **vocabulário do arquétipo do ADR-0023**, não um nome de produto.
3. **Forma certa acima de compatibilidade do data plane.** Ao evoluir o backend, priorizamos o design correto do control plane mesmo que isso quebre um data plane já implementado. O data plane migra depois; as mudanças cross-repo ficam documentadas (ex.: `docs/game-models/queue_arena-dataplane-contract.md`). Confirmado pelo dono do projeto em 2026-07-06.

## Padrão de nomenclatura

```
go-game-<archetype>-dataplane      # um por arquétipo (Unity)
go-game-control-plane              # o backend Go (único)  ← rename proposto
```

- `<archetype>` = o enum do arquétipo (ADR-0023) em **kebab-case**: `queue-arena`, `battle-royale`, `coop-mission`, `host-world-dropin`.
- Sufixo `-dataplane` fixo (par de `control-plane`).

## Mapa de repositórios

| Arquétipo (ADR-0023) | Repo data plane (proposto) | Repo atual | Status |
|---|---|---|---|
| `queue_arena` | `go-game-queue-arena-dataplane` | `go-game-dataplane` | ✅ existe (exemplo de referência; jogo "Packet Arena") — **renomear** |
| `battle_royale` | `go-game-battle-royale-dataplane` | — | ⬜ planejado (criar quando atacar o arquétipo) |
| `coop_mission` | `go-game-coop-mission-dataplane` | — | ⬜ planejado |
| `host_world_dropin` | `go-game-host-world-dropin-dataplane` | — | ⬜ planejado |
| — (control plane) | `go-game-control-plane` *(ou `go-game-multiplayer-backend`)* | `go-game-backend` | ✅ existe — **renomear** (ver decisão abaixo) |

> `go-game-unity` (repo vazio, órfão): resquício do plano antigo (memória "dataplane-repo"). Data plane real virou `go-game-dataplane`. **Arquivar/remover** no rename.

```mermaid
flowchart TB
  CP["go-game-control-plane (Go)<br/>identity · matchmaking · session · leaderboard · ...<br/>orientado por perfil (ADR-0023)"]
  QA["go-game-queue-arena-dataplane<br/>(Unity) ✅ existe"]
  BR["go-game-battle-royale-dataplane<br/>⬜ planejado"]
  CM["go-game-coop-mission-dataplane<br/>⬜ planejado"]
  HW["go-game-host-world-dropin-dataplane<br/>⬜ planejado"]
  CP -- "perfil queue_arena" --> QA
  CP -- "perfil battle_royale" --> BR
  CP -- "perfil coop_mission" --> CM
  CP -- "perfil host_world_dropin" --> HW
```

## Decisão pendente — nome do control plane

O dono levantou dois candidatos; opção fechada = próxima sessão.

| Opção | Prós | Contras |
|---|---|---|
| `go-game-control-plane` *(recomendado)* | simétrico com `…-dataplane`; casa com ADR-0010; preciso | "control plane" é jargão — menos óbvio para quem chega |
| `go-game-multiplayer-backend` | "backend" é reconhecível; "multiplayer" ajuda descoberta | assimétrico com os data planes; "backend" é vago |

**Recomendação:** `go-game-control-plane` — a simetria control/data plane é o eixo organizador da topologia e já é o vocabulário dos ADRs.

## Ao executar o rename (checklist futuro — não fazer agora)

Renomear toca o module path Go e várias referências. Quando decidido:

1. **Repos GitHub:** renomear (GitHub redireciona o antigo por um tempo) e `git remote set-url`.
2. **Module path Go** (`go-game-backend` → novo): `go.mod` + todos os imports `github.com/vitormartins1/go-game-backend/...` (ADR-0001, módulo único). Impacta o stub gerado que o data plane consome (proto go_package).
3. **Referências em docs do backend** (ajustar os links relativos `../go-game-dataplane` e menções): `docs/ARCHITECTURE.md`, `docs/NEXT-STEPS.md`, `docs/ROADMAP.md`, `docs/UNITY-INTEGRATION.md`, `docs/game-models/*`, ADRs 0010/0011/0012/0020/0021/0022/0023 e `docs/adr/README.md`.
4. **Referências no data plane** (`go-game-dataplane`): README, CLAUDE.md, ROADMAP, specs, links `../go-game-backend`.
5. **Memórias** do assistente que citam nomes de repo (`dataplane-repo`, etc.).
6. **CI/CD e paths** que embutam o nome do repo/módulo.

> Sugestão de sequência: renomear os repos → module path → varredura de referências → validar build/CI. Feito de uma vez para não conviver com nomes mistos.

## Próximos passos

- [ ] Fechar o nome do control plane (tabela acima) — **decisão do dono**.
- [ ] Criar os repos data plane planejados conforme cada arquétipo for atacado (pastas/repos já com o nome do padrão).
- [ ] Executar o rename (checklist) numa janela dedicada.
- [ ] (Opcional) Registrar como **ADR** se quiser tornar a convenção formal — hoje é proposta neste doc.
