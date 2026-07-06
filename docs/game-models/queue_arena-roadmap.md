# Roadmap — specs restantes do `queue_arena` (ADR-0023)

> **Propósito:** planejar em lote as fatias que faltam para o arquétipo `queue_arena` estar **completo e refletido** (profundidade no backend + visível/editável no console), evitando re-derivar contexto a cada fatia. Estado do que está **feito** vive na tabela "Estado da implementação" do [README](./README.md). Aqui só o **que falta**.
>
> Ordem recomendada: **B1 → B2 → A1 → A2**. `A3` e o bloco `C` (outros arquétipos) são **próxima etapa**, fora deste roadmap.

## Acordo de trabalho (economia de contexto)

- Uma fatia por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff` (como as 7 já feitas).
- **Fonte única de estado:** tabela do `README` + este doc. Não re-explorar o que já está documentado.
- SDD enxuto: `spec.md` curta + `plan.md`/`tasks.md` diretos; implementar; testes verdes; merge.
- Admin-UI é TS (ciclo build/test leve). Backend Go usa testcontainers só onde há Postgres.

---

## B1 — Perfil de multiplayer no console  ⭐ (porta de entrada, maior)

**Objetivo:** cadastrar/editar o perfil de multiplayer de um jogo pelo admin-ui (hoje só via API). Torna o ADR-0023 visível e usável.

**Specs:** `admin-bff/012-multiplayer-profile` + `admin-ui/015-multiplayer-profile`. (game-registry/006 já tem `GET`/`PUT /games/{id}/multiplayer-profile` — **sem mudança no backend de domínio**.)

**admin-bff (012):** passthrough `GET`/`PUT /api/games/{id}/multiplayer-profile` → game-registry (reusa `internal/upstream` + adapter gameregistry já existente do rename). Repassa 400 `ErrInvalidProfileParams` como erro estruturado.

**admin-ui (015):** seção "Multiplayer" na aba **Settings** (hoje só rename) ou aba nova. Formulário com campos **condicionais** (espelha a validação da game-registry/006):
- `archetype`: só `queue_arena` por ora (Select preparado para crescer).
- `structure`: `ffa` | `teams` (SegmentedControl).
  - `ffa` → `playerCount` (≥2).
  - `teams` → `teamCount` (≥2) + `teamSize` (≥1).
- `winCondition`: `first_to_score` | `highest_at_time` | `objective` | `elimination`.
  - `first_to_score` → `scoreTarget` (>0); `highest_at_time` → `durationSeconds` (>0).
- `matchStructure`: `single` | `best_of_rounds` → `roundsToWin` (>0).
- `allowDraw` (Switch); `scoringStats` (TagsInput; cada item `^[a-z][a-z0-9_]*$`).
- `topology`: `dedicated` | `p2p` (Select + tooltip; `p2p` = slot, não implementado — ADR futuro).
- Jogo sem perfil → estado "não configurado" + criar. Validação client-side; servidor é a verdade.

**Depende de:** nada novo. **Tamanho:** L (form condicional + validação + passthrough). **Destrava:** B2b e as caudas de UI de A1/A2 (que precisam ler `scoringStats`/estrutura do jogo).

---

## B2 — Surfacing barato do que o backend já produz  (rápido, alto valor visual)

Divisível em duas fatias pequenas e independentes:

**B2a — Analytics: derrotas/empates** (`admin-ui/016`)
O JSON de analytics já traz `losses`/`draws` (fatia analytics/003, aditivo — passa pelo bff sem mudança). Mostrar nos StatCards da aba Analytics (overview + por jogo): vitórias/derrotas/empates. **Só admin-ui.** **Tamanho:** S.

**B2b — Leaderboard: seletor de board** (`admin-bff/013` + `admin-ui/017`)
O leaderboard-service já serve `?board` (score + um por stat — fatia leaderboard/003). admin-bff repassa o param `board`; admin-ui ganha um Select de board na aba Leaderboards. Boards disponíveis = `score` + `scoringStats` do perfil do jogo (**lê o perfil — leve dependência de B1/GET**). **Tamanho:** S–M.

---

## A1 — Conquistas por `stats`/`teamId`  (aprofunda o colher)

**Objetivo:** conquistas com critério por métrica (ex.: "10 kills") e por time, usando `stats`/`teamId` que o outcome já carrega.

**Spec:** `achievements/008-stat-criteria` (+ cauda de UI no catálogo, `admin-ui/018`).

**Escopo:**
- Modelo do catálogo (CRUD, achievements/006) ganha um **tipo de critério por stat**: `{ stat_key, threshold, mode: cumulative|per_match }`. Hoje os gatilhos são eventos `match_played/won/lost/drawn` (spec 007).
- Consumer lê `stats` do outcome e avança progresso por métrica (cumulativo entre partidas ou por partida). Idempotência por matchID mantida (tabela `processed_matches`).
- (opcional) conquistas condicionadas a `teamId` / vitória por time.
- Cauda admin-ui: editor do catálogo aceita o novo tipo de critério.

**Depende de:** outcome com stats (✅). **Tamanho:** M–L (toca modelo + consumer + UI).

---

## A2 — Leaderboard acumulativo / por outcome  (aprofunda o colher)

**Objetivo:** rankings acumulativos além do highscore-por-partida atual: ranking de **vitórias** e **totais de stat** (ex.: total de kills na temporada).

**Spec:** `leaderboard/004-cumulative-boards`.

**Escopo:**
- Board ganha **modo de agregação**: `highscore` (atual, `ZADD GT`) vs `cumulative` (`ZINCRBY`).
- Consumer: em `win` incrementa o board de vitórias; por stat declarado, incrementa o total (`ZINCRBY`). Idempotência: `ZINCRBY` **não** é idempotente de graça → precisa de guarda por matchID (ex.: set de matches processados por board, ou dedupe no consumer). **Gotcha a resolver no plan.**
- Read: `?board=wins` / `?board=kills_total` etc.

**Depende de:** outcome (✅). **Tamanho:** M. **Atenção:** idempotência do `ZINCRBY` (diferente do `ZADD GT`, que é idempotente).

---

## Fora deste roadmap (próxima etapa)

- **A3 — `winCondition`/`mode`/`rounds` no gameplay:** majoritariamente **data plane (Unity)**, cross-repo; o backend já declara no perfil. Ver `queue_arena-dataplane-contract.md`.
- **C — outros arquétipos** (`battle_royale`, `coop_mission`, `host_world_dropin`): cada um puxa modelo de resultado próprio (placement/collective/role_event) e seu **repo de data plane** (ver `REPO-TOPOLOGY.md`). Etapa seguinte.

## Resumo de sequência

| Fatia | Specs | Tamanho | Depende |
|---|---|---|---|
| B1 — perfil no console | admin-bff/012 + admin-ui/015 | L | — |
| B2a — analytics loss/draw | admin-ui/016 | S | — |
| B2b — seletor de board | admin-bff/013 + admin-ui/017 | S–M | B1 (GET perfil) |
| A1 — conquistas por stats/teamId | achievements/008 (+admin-ui/018) | M–L | — |
| A2 — leaderboard acumulativo | leaderboard/004 | M | — |

Concluídas B1+B2+A1+A2, o `queue_arena` está **completo** (backend + console). Próxima etapa: bloco C.
