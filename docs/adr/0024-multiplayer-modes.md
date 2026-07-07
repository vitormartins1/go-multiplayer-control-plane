# ADR-0024: Modos de jogo — perfis de multiplayer nomeados por jogo + session manifest

**Status:** Accepted
**Data:** 2026-07-07
**Serviço(s) afetado(s):** game-registry, matchmaking-service, session-service, achievements-service, leaderboard-service, analytics-service, admin-bff, admin-ui, data planes (Unity)
**Relaciona:** ADR-0023 (perfil de multiplayer por arquétipo), ADR-0017 (game-registry como fonte canônica), ADR-0010 (control/data plane), ADR-0020 (fan-out), ADR-0021 (analytics)

---

## Contexto

O ADR-0023 fez o jogo declarar **um** perfil de multiplayer (arquétipo + parâmetros) e provou o modelo com 4 arquétipos ponta a ponta no control plane. Mas a relação **jogo 1—1 perfil** não sobrevive ao contato com jogos reais — a evidência vem dos próprios jogos de referência das fichas (`docs/game-models/`):

- **Elden Ring** = `host_world_dropin` (o mundo) **+ `queue_arena`** (o Coliseu é PvP por fila);
- **CoD** = `queue_arena` (TDM, S&D, …) **+ `battle_royale`** (Warzone);
- e mesmo dentro de um arquétipo, um jogo tem **vários modos**: TDM e S&D são dois `queue_arena` com parâmetros diferentes (ficha 05 já havia registrado o gap "modo de partida como preset").

Ou seja: **jogo real = conjunto de modos, cada modo com uma forma**. Além disso, o review pós-ADR-0023 (2026-07-07) apontou três lacunas que convergem para a mesma solução:

1. **Perfil mutável sem versionamento** — trocar o perfil com tickets na fila ou sessões abertas produz estados mistos (consequência já prevista no ADR-0023, sem solução até aqui).
2. **O data plane não lê os parâmetros declarados** — `winCondition` etc. estão no perfil, mas o server Unity segue com `first-to-5` hardcoded; cada ajuste de regra é mudança cross-repo.
3. **O session não valida o `kind` do outcome contra o arquétipo do jogo** — um server de `queue_arena` pode reportar `collective` e poluir os rollups silenciosamente.

## Alternativas Consideradas

### Opção A: `mode` como enum/preset dentro dos params de cada arquétipo
- **Prós:** mudança pequena e local (era a direção original da ficha 05).
- **Contras:** não cruza arquétipos (Warzone + TDM não cabem num único perfil `queue_arena`); incha os params de cada arquétipo com variações; dois modos do mesmo arquétipo teriam que duplicar campos dentro de um único registro. Rejeitada.

### Opção B: **Perfis nomeados 1—N por jogo + snapshot na sessão + session manifest** (escolhida)
- **Prós:** "modo de jogo" vira exatamente "perfil nomeado" — reusa TODO o modelo do ADR-0023 (arquétipo + params + validação) sem alterá-lo; cruza arquétipos naturalmente; o snapshot resolve o versionamento; o manifest fecha o gap dos params não lidos; migração barata (perfil atual → modo `default`).
- **Contras:** filas/cache do matchmaking ganham uma dimensão; UI de modos no console; a segmentação dos sinks por modo vira uma decisão pendente (adiada por evidência).

### Opção C: Serviço próprio de modos/rulesets
- **Contras:** o modo é um atributo do **jogo**, que vive no game-registry (mesmo raciocínio que rejeitou a Opção D do ADR-0023 — fragmentaria a fonte canônica sem ganho). Rejeitada.

## Decisão

Adotamos a **Opção B**, em cinco pontos:

1. **Jogo 1—N perfis nomeados.** Cada jogo tem uma coleção de perfis de multiplayer, cada um com `modeId` (slug estável, ex.: `default`, `tdm`, `ranked`), `displayName` e o corpo do ADR-0023 (`archetype`, `topology`, `params` — modelo e validação **inalterados**). O perfil único atual migra para o modo **`default`**; o endpoint atual (`/games/{id}/multiplayer-profile`) passa a ser alias do `default` para compatibilidade até o M1.

2. **Matchmaking por (jogo, modo).** `FindMatch` ganha `mode_id` **opcional** (ausente = `default`); filas e cache de formação passam a ser por `(gameID, modeID)`. O mesmo vale para as entradas do `host_world_dropin` (`OpenWorld`/`Invade`/sinais): o mundo aberto pertence a um modo.

3. **Session congela um snapshot do perfil.** Ao criar a sessão (`Create`/`OpenWorld`), o session grava `modeId` + **snapshot imutável** do perfil usado — mudar o perfil no registry não afeta sessões/partidas em voo (resolve a lacuna de versionamento com um mecanismo só). Com o perfil em mãos, o `ReportResult` passa a **validar o `kind` do outcome contra o arquétipo do modo** (fecha a lacuna de integridade #3).

4. **Session manifest — o data plane lê a configuração, não a hardcoda.** O game server obtém, no início da sessão, um manifest `{archetype, modeId, params (snapshot), profileHash}` e lê **só as chaves que conhece**. Consequências: o `winCondition`/`missions`/`respawnMode` declarados finalmente orientam o gameplay; **um mesmo binário Unity serve N modos**; params novos deixam de ser mudança de contrato (viram chave nova no blob). O mecanismo de entrega (RPC `GetSessionConfig(match_id)` no session vs. annotations na alocação Agones) é decisão de `plan.md` da fatia M1, não deste ADR.

5. **Fan-out ganha `modeId` (aditivo); segmentação dos sinks é adiada.** O envelope do `match-results` carrega `modeId` desde o primeiro dia (consumidores ignoram até precisar). Leaderboard/analytics/achievements **continuam agregando por jogo**; segmentar por modo (ranked vs. casual, seasons) só quando houver demanda real — a dimensão já estará nos eventos.

**Fatiamento:** roadmap `docs/game-models/multiplayer-modes-roadmap.md` — **M0** (registry + console) → **M1** (matchmaking/session/token/manifest/envelope) → **M2** (segmentação de sinks, gated por evidência).

## Consequências

### Positivas
- Jogos com múltiplos modos entram por **cadastro**, inclusive cruzando arquétipos — a tese de genericidade (ADR-0008) deixa de ter o teto do 1—1.
- Snapshot por sessão elimina a classe de bugs "perfil mudou no meio" e dá auditoria de graça.
- Manifest desengessa a fronteira control↔data plane: menos mudanças cross-repo, mesmo binário para N modos.
- `modeId` no envelope prepara ranked/seasons sem retrabalho nos produtores.

### Negativas / Trade-offs
- Combinatória de teste cresce (arquétipo × modo × serviço).
- Console precisa de UI de coleção (lista de modos + form por modo) em vez de um form único.
- Quando a segmentação por modo chegar aos sinks (M2), leaderboards/rollups ganham dimensionalidade (chaves e migrations novas) — adiado deliberadamente.
- Alias `multiplayer-profile` → `default` é dívida de compat a remover quando matchmaking/session migrarem (M1).

### Neutras / Implicações
- **SDK Unity compartilhado** (handshake + leitura do manifest como pacote UPM) é **válido e desejado, mas fica para o futuro** — registrado como princípio 5 do `REPO-TOPOLOGY.md`; até lá, cada data plane de referência copia o handshake.
- Os data planes de referência continuam **por arquétipo**; um jogo real multi-modo é uma categoria própria de repo (princípio 4 do `REPO-TOPOLOGY.md`).
- Specs por fatia: `game-registry/009` (M0) em diante; contratos por arquétipo ganham o claim `mode_id` no connect token quando o M1 chegar.

## Referências
- Review do ADR-0023 (2026-07-07) — lacunas #1–#3 do contexto.
- `docs/game-models/` — fichas 01–05 (em especial 05: gênero ≠ arquétipo; gap "modo como preset", absorvido por este ADR).
- ADR-0023, ADR-0017, ADR-0010, ADR-0020, ADR-0021.
