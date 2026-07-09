# Roadmap — modos de jogo (perfis nomeados) + session manifest (ADR-0024)

> **Propósito:** planejar em lote as fatias que levam o modelo de perfil do **1—1** (ADR-0023) para **jogo 1—N modos** (ADR-0024), com snapshot por sessão e o **session manifest** que faz o data plane ler a configuração em vez de hardcodá-la. Espelha os roadmaps por arquétipo: fonte única de estado é a tabela "Estado da implementação" do [README](./README.md); aqui só o plano.
>
> **Origem:** review pós-ADR-0023 (2026-07-07) + evidência das fichas (Elden Ring = `host_world_dropin` + `queue_arena` no Coliseu; CoD = `queue_arena` + `battle_royale`; ficha 05 = TDM/S&D são dois `queue_arena` com params diferentes). "Modo de jogo" = **perfil nomeado** — o corpo do perfil (arquétipo + params + validação) **não muda**.
>
> Ordem: **M0 → M1 → M2**. M2 é **gated por evidência** (não implementar sem demanda real).

## Ponto de partida (o que já está na `main`)

- ✅ Perfil polimórfico por arquétipo completo (4 arquétipos, 4 kinds de outcome, sinks por `kind`) — ADR-0023.
- ✅ O envelope de fan-out é versionado/discriminado e os consumidores toleram campos novos (aditivo barato).
- ⬜ Relação jogo↔perfil é 1—1 (coluna JSONB única, `PUT/GET /games/{id}/multiplayer-profile`); matchmaking/session leem "o perfil do jogo" sem noção de modo; data plane não lê params (first-to-5 hardcoded); `ReportResult` não valida `kind` × arquétipo.

## Decisões já tomadas no ADR (não rediscutir nas specs)

1. Modo = perfil nomeado (`modeId` slug estável + `displayName` + corpo do ADR-0023). Migração: perfil atual → modo **`default`**; endpoint antigo vira alias do `default` até o M1.
2. `mode_id` **opcional** nas entradas (ausente = `default`) — retrocompat total com clientes atuais.
3. Session grava `modeId` + **snapshot imutável** do perfil; `ReportResult` valida `kind` × arquétipo do modo.
4. Manifest `{archetype, modeId, params, profileHash}` para o data plane; **mecanismo de entrega é decisão do plan do M1** (RPC `GetSessionConfig` vs. annotations Agones).
5. Envelope ganha `modeId` aditivo; sinks seguem por jogo até o M2.

---

## M0 — Modos nomeados no game-registry + console

**Objetivo:** o jogo cadastra N perfis nomeados; o console lista/edita modos; tudo que existe continua funcionando via alias `default`.

**Specs:** `game-registry/009-multiplayer-modes` + admin-ui (numeração livre).

**Escopo:**
- **game-registry:** coleção `multiplayer-profiles` (`GET` lista, `GET/PUT/DELETE /{modeId}`); `modeId` slug estável (regras na spec), `default` não-removível; validação por arquétipo **reusada** intacta; persistência na coluna JSONB existente com **upgrade lazy** do formato antigo (objeto único → mapa de modos) — sem migration SQL; endpoint antigo `multiplayer-profile` = alias do `default`.
- **admin-ui:** aba Configurações vira **lista de modos** (badge de arquétipo por modo) + form atual reusado por modo; **admin-bff:** passthrough cru (`json.RawMessage`), provavelmente zero mudança (achado da BR4).

**Depende de:** nada. **Tamanho:** M. **Destrava:** M1.

## M1 — Matchmaking/session por modo + snapshot + manifest

**Objetivo:** o modo flui ponta a ponta: ticket → formação → sessão (snapshot) → connect token → manifest → resultado validado → envelope.

**Specs:** `matchmaking/010` + `session/017` (+ atualização dos 4 contratos de data plane).

**Escopo:**
- **matchmaking:** `mode_id` opcional em `FindMatch` (e nas entradas do `host_world_dropin`); fila e `profilecache` por `(gameID, modeID)`; `registryclient` busca `/multiplayer-profiles/{modeId}`.
- **session:** `Create`/`OpenWorld` recebem `modeID`; sessão grava `modeId` + snapshot do perfil; claim **`mode_id`** no connect token; **manifest** para o data plane (mecanismo decidido no plan); `ReportResult` valida `kind` × arquétipo do snapshot (`ErrOutcomeMismatch` novo); envelope de fan-out ganha `modeId`.
- **contratos:** os 4 `*-dataplane-contract.md` ganham o claim `mode_id` + a seção do manifest (o data plane novo já nasce lendo).

**Depende de:** M0. **Tamanho:** M–L (o manifest é o pedaço novo; o resto é rosca de dimensão).

## M2 — Segmentação dos sinks por modo (gated por evidência)

**Objetivo:** leaderboards/rollups/conquistas **por modo** quando um caso real exigir (ranked vs. casual, seasons — backlog do leaderboard).

**Escopo (quando ativar):** chave `(gameID, modeID)` nos boards do leaderboard, rollups do analytics e (se fizer sentido) critérios de conquista por modo. O `modeId` **já estará** em todos os envelopes desde o M1 — é só dimensionar os consumidores, sem retrabalho nos produtores.

**Gate:** não implementar antes de existir um jogo/validação que precise de dois modos com progressão separada.

> **Evidência para o gate (2026-07-08):** o Capoeira Souls tem **7 modos cadastrados** (6 `host_world_dropin` + 1 `queue_arena`; catálogo em `unity-host-world-dropin-data-plane/docs/MODES.md`) e o analytics local populado com 96 participation events dos 6 modos — **todos colapsando num único rollup `(gameId, role, event)`**: não dá para comparar modos no console (qual retém phantoms? qual está deserto?). A dimensão `modeId` nos rollups do analytics é o primeiro pedaço do M2 com demanda concreta; combina com a dimensão `outcome` (follow-up #5 do arquétipo) para taxa de sucesso de invasão por modo.

---

## Aberturas declaradas, adiadas

- **SDK Unity compartilhado** (handshake + manifest como pacote UPM) — princípio 5 do [REPO-TOPOLOGY.md](../REPO-TOPOLOGY.md); quando o primeiro jogo final entrar em cena.
- **Remoção do alias `multiplayer-profile`** quando M1 estiver na main e nenhum consumidor usar o endpoint antigo.
- **Skill/MMR, party, região** — dimensões de matchmaking ortogonais a modo (log de generalizações).
- **Topologia `p2p`** — segue diferida a ADR próprio (ADR-0023 §4).
