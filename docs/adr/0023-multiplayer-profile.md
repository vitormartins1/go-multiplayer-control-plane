# ADR-0023: Perfil de multiplayer por jogo — arquétipo + parâmetros orientando o control plane

**Status:** Accepted
**Data:** 2026-07-06
**Serviço(s) afetado(s):** game-registry, matchmaking-service, session-service, achievements-service, leaderboard-service, analytics-service, admin-bff, admin-ui, data plane (Unity)
**Relaciona:** ADR-0017 (multi-tenant/game-registry), ADR-0010 (control/data plane), ADR-0011/0012 (Unity/Agones), ADR-0020 (fan-out do resultado), ADR-0021 (analytics)

---

## Contexto

O control plane assume hoje **um único arquétipo de multiplayer** implícito: partida simétrica de N jogadores, formada por fila FIFO, terminando com um `PlayerResult{ PlayerID; Score int64; Won bool }` (session `001`/`002`) → fan-out `match_played`/`match_won`. As regras estão **hardcoded** (ex.: o data plane usa `first-to-5` fixo, spec `006-match-result`).

O levantamento em `docs/game-models/` (5 fichas, engenharia reversa de jogos reais) mostrou que esse arquétipo cobre bem só uma fatia. Quatro **formas** distintas de multiplayer real-time apareceram, cada uma quebrando o modelo num eixo diferente:

- **Arena competitiva** (ficha 02, o veículo atual) — WIN/LOSS/**DRAW**, por jogador ou time.
- **Battle royale** (03) — **colocação ordinal** (1º..Nº) + respawn configurável.
- **Souls / drop-in no mundo do host** (01) — outcome **por papel/evento**, sessão host-cêntrica.
- **Co-op PvE de missão** (04) — outcome **coletivo** (`MISSION_SUCCESS/FAILURE`).

A ficha 05 (shooters) estabeleceu o **princípio: gênero ≠ arquétipo** — FPS/TPS e muitos gêneros (MOBA, esporte, luta 1v1) colapsam no mesmo arquétipo com parâmetros diferentes; a câmera/tema é data plane. O denominador comum das 5 fichas: o **modelo de resultado é o gargalo central**, e cada jogo precisa **declarar sua forma** em algum lugar canônico. Precisamos decidir **onde** isso é declarado e **como** o control plane passa a ser orientado por essa declaração — antes de expandir os serviços para novos jogos.

## Alternativas Consideradas

### Opção A: Status quo — um arquétipo hardcoded
- **Prós:** zero trabalho; simples; serve o veículo atual.
- **Contras:** não serve 3 dos 4 arquétipos; regras no código, não no cadastro do jogo; cada novo jogo exigiria mexer no backend. Contradiz a tese de genericidade (ADR-0008).

### Opção B: Perfil de multiplayer por jogo = **arquétipo (enum) + parâmetros** (escolhida)
- **Prós:** o jogo declara sua forma no game-registry (fonte canônica, ADR-0017); matchmaking/session/resultado leem o perfil e se comportam de acordo; o data plane "herda o caminho" pré-configurado; expansível por evidência.
- **Contras:** game-registry, matchmaking e session ganham complexidade; o contrato de resultado precisa se tornar polimórfico (muda `ReportResult`/fan-out).

### Opção C: Motor de regras/DSL totalmente genérico
- **Prós:** cobriria qualquer mecânica imaginável sem novos arquétipos.
- **Contras:** over-engineering clássico; difícil de validar, versionar e explicar; empurra complexidade de gameplay para o control plane (que não deve conhecer regras de jogo — ADR-0010). Descartada.

### Opção D: Um "ruleset-service" separado, fora do game-registry
- **Prós:** separação de responsabilidades.
- **Contras:** o perfil é um atributo do **jogo**, que já vive no game-registry; um serviço à parte fragmentaria a fonte canônica sem ganho no MVP. Descartada (reavaliar se o perfil crescer muito).

## Decisão

Adotamos a **Opção B**. O **game-registry** passa a guardar, por jogo, um **perfil de multiplayer** composto de:

1. **Arquétipo** (enum) — a *forma* da sessão/matchmaking/resultado. Conjunto **inicial fechado** (expandível por evidência, **sem DSL**):
   - `queue_arena` — fila/lados NvN ou FFA, sessão simétrica, resultado WIN/LOSS/DRAW + stats. **Absorve shooters, MOBA, esporte, luta** (gênero ≠ arquétipo).
   - `battle_royale` — lobby grande, eliminação progressiva, colocação ordinal, respawn configurável.
   - `host_world_dropin` — sessão host-cêntrica, join-in-progress com papéis (co-op/invasor), outcome por papel/evento.
   - `coop_mission` — time vs. ambiente, join-in-progress, outcome coletivo.

2. **Parâmetros por arquétipo** — ex.: `queue_arena{ mode, matchStructure(single|best_of_rounds N), teamSize, winCondition, allowDraw, scoringStats[] }`; `battle_royale{ lobbySize, teamSize, respawnMode, scoring }`; `host_world_dropin{ roles[], capacityPerRole }`; `coop_mission{ teamSize, botfill, missions[], difficulties[] }`. **Schema fino fica para as specs**, não para este ADR.

3. **Modelo de resultado polimórfico** — `PlayerResult{Score,Won}` é substituído por um **outcome** cuja forma depende do arquétipo: `win_loss_draw` (+ stats), `placement` (ordinal), `collective` (por sessão), `role_event` (por papel/evento). É a **mudança mais impactante**; session/leaderboard/achievements/analytics passam a consumir esse outcome.

4. **Topologia** (`dedicated` | `p2p`) como **campo do perfil**. O control plane é **agnóstico de transporte** via uma Port `SessionProvisioner` (adapters `agones` / `p2p`). **`dedicated` é implementado primeiro** (já modelado, ADR-0011/0012, e é a tese de valor: autoridade neutra). A **implementação de `p2p`** (transport + relay/NAT, provável repo Unity à parte) é **diferida a um ADR próprio** — aqui fica apenas o *slot* no modelo.

**Orientação por perfil:** matchmaking e session deixam de assumir um formato fixo e passam a **ler o perfil do jogo** (via game-registry) para decidir formação de partida, forma da sessão e interpretação do resultado.

## Consequências

### Positivas
- Genericidade **real e validada** por 5 jogos reais; novos jogos entram por **cadastro**, não por código.
- O data plane herda um contrato pré-configurado (liga control ↔ data plane).
- Resultado rico habilita leaderboards/achievements melhores (K/D, MVP, colocação, missões).
- Topologia flexível por jogo, mantendo a autoridade neutra como default.

### Negativas / Trade-offs
- **Contrato de resultado muda:** `ReportResult` (gRPC, session) e o **fan-out** (ADR-0020, SNS→SQS) e o **analytics** (ADR-0021) precisam acomodar outcome polimórfico — migração e versionamento de contrato.
- **Acoplamento** de matchmaking/session ao perfil do game-registry (já validam `game_id`, mas agora dependem do conteúdo do perfil — exige cache/resiliência).
- game-registry cresce (schema de perfil, validação por arquétipo, versionamento do perfil).
- Mais superfície de teste (cada arquétipo × cada serviço).

### Neutras / Implicações
- Virão **specs por arquétipo** (começando por `queue_arena`, que já é o mais próximo do atual).
- Provável **ADR/spec dedicado ao "modelo de resultado polimórfico"** (dado o impacto no fan-out/analytics).
- **ADR futuro** para a implementação da topologia `p2p` (transport + relay + repo Unity).
- admin-ui/admin-bff ganham a UI de cadastro do perfil (seletor de arquétipo + campos por arquétipo + tooltips de topologia).
- O perfil precisará de **versionamento** (mudar o perfil de um jogo em produção afeta partidas/leaderboards).

## Referências
- `docs/game-models/` — README (taxonomia, eixos, log de generalizações) e fichas 01–05.
- ADR-0017 (game-registry como fonte canônica), ADR-0010 (control/data plane), ADR-0020 (fan-out), ADR-0021 (analytics), ADR-0008 (tese de genericidade).
