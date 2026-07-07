# Plan: 009-coop-mission-team

**Serviço:** matchmaking-service · **Spec:** [spec.md](./spec.md) · **ADR:** 0023 · **Roadmap:** C1

## Objetivo

Ensinar o `registryclient.Format` a derivar a formação do `coop_mission`: **um único time** de `teamSize`. Reuso máximo — a mudança fica inteira no adapter; `composeTeams`, o domínio e o session-service **não mudam**.

## Decisões técnicas

### D1 — Co-op = `StructureTeams` com `TeamCount=1` (não uma estrutura nova)

- O ramo `StructureTeams` do `composeTeams` já emite, para `TeamCount=1`, um `TeamAssignment` (`team-1`) com todos os jogadores. Reusá-lo evita um `StructureCoop` novo + ramo + testes. O rótulo `team-1` é cosmético (há um só time); o `team_id` compartilhado é o que importa (vai no connect token e no resultado coletivo, C2). Registrado na spec como decisão consciente.

### D2 — Validação `teamSize ≥ 2` no `Format`

- Espelha o `case archetypeBattleRoyale` (que valida `teamSize > 0`). Aqui `< 2` → `ErrProfileUnavailable` (coerente com a regra do C0). `Size = teamSize`; o guard geral `format.Size <= 0` do fim do `Format` continua valendo como rede de segurança.

### D3 — `profileResponse` já basta

- `p.Params.TeamSize` já existe (usado por `queue_arena teams` e `battle_royale`). Nenhum campo novo no `profileResponse`.

## Mudanças por camada (arquivos)

| Arquivo | Mudança |
|---|---|
| `internal/registryclient/client.go` | const `archetypeCoopMission = "coop_mission"`; `case archetypeCoopMission` no `switch` do `Format` (valida `teamSize ≥ 2`; monta `MatchFormat{teams, TeamCount:1, TeamSize, Size:teamSize}`); atualizar o doc-comment do `Format` |
| `internal/service/matchmaker.go` | **sem mudança** (composeTeams reusa StructureTeams) |
| `internal/domain/…` | **sem mudança** |

## Testes

- **registryclient** (`client_test.go`, `httptest`): `coop_mission teamSize=4` → `{teams, TeamCount:1, TeamSize:4, Size:4}`; `teamSize=1` → `ErrProfileUnavailable`; **regressão** implícita (os testes de `queue_arena`/`battle_royale`/`host_world_dropin` já existentes seguem passando).
- **matchmaker** (`matchmaker_test.go`): fila de um jogo `coop_mission` (format `teams`/`TeamCount:1`/`Size:teamSize`) enche `teamSize` tickets → 1 grupo, `Provision` recebe 1 assignment `team-1` com os N jogadores (mock do provisioner). Reusa o harness existente.

## Observabilidade

Sem trabalho novo: as métricas do matchmaker (`matches_total`/`matches_failed`/`queue_size`) são agnósticas de arquétipo. Sem exporter novo (stateless).

## Riscos / fora do escopo

- **Risco baixo:** um `case` no adapter, reuso total do resto. O teste do matchmaker garante que o time único chega ao provisioner.
- Bot-fill / lobby-timeout, join-in-progress, resultado coletivo (C2) ficam fora.
