# Roadmap — arquétipo `host_world_dropin` ponta a ponta (ADR-0023)

> **Propósito:** planejar em lote as fatias que levam o arquétipo `host_world_dropin` de **declaração** (o que já existe na `main`) a **comportamento ponta a ponta no control plane** — o mundo do host abre, phantoms co-op/invasor são descobertos e injetados por papel, o server do mundo reporta eventos por participante e o console mostra. Espelha o [`queue_arena-roadmap.md`](./queue_arena-roadmap.md): fonte única de estado é a tabela "Estado da implementação" do [README](./README.md); aqui só o **que falta**.
>
> **Origem do modelo:** ficha [01 — Soulslike](./01-soulslike-invasion-coop.md). Leia-a antes de abrir a primeira spec: o arquétipo **quebra o modelo atual em quase todos os eixos** (não há partida simétrica formada por fila, não há placar terminal, a sessão é o mundo persistente de um host, phantoms entram/saem no meio com papéis assimétricos).
>
> Ordem recomendada: **H0 → H1a → H1b → H2 → H3**. A tabela de topologia `p2p` e os demais arquétipos (`battle_royale`, `coop_mission`) ficam fora deste roadmap.

## Ponto de partida (o que já está na `main`)

- ✅ **Declaração** (commit `6fbb3cf`): `HostWorldDropInParams` no game-registry (`EnabledRoles` `coop`/`invader`, `CoopSlots`, `InvaderSlots`, `LevelRangeEnabled`, `WeaponTierEnabled`, `PasswordEnabled`, `InvasionEnabled`) + validação + persistência JSONB + form condicional no console (aba Configurações, com Fieldsets/tooltips).
- ⬜ **Nenhum serviço reage ao arquétipo.** matchmaking (queue-form FIFO), session (sessão nasce de match, 1 `ReportResult` terminal) e o resultado (`MatchOutcome`/`WinLossDraw`) continuam presos ao `queue_arena`.

O jogo hoje **declara** um perfil `host_world_dropin`, mas ninguém o consome. Este roadmap dá comportamento a ele.

## Acordo de trabalho (economia de contexto)

- Uma fatia por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff` (como o `queue_arena`).
- **Fonte única de estado:** tabela do [README](./README.md) + este doc. Não re-explorar o que já está documentado.
- SDD enxuto: `spec.md` curta + `plan.md`/`tasks.md` diretos; implementar; testes verdes (`-race`, ≥80% em `service/`/`domain/`); merge.
- Cada serviço tocado passa pela observabilidade **antes de fechar a fatia** (métricas RED, `docs/OBSERVABILITY.md`).

## Tensões de modelagem a decidir no `plan.md` (não bloquear agora)

Três decisões estruturais que atravessam as fatias. **Recomendação anotada; confirmar por escrito no plan da fatia dona.**

1. **Sessão sem match (dona de H0).** Hoje `SessionService.Create(matchID, gameID, teams)` exige um match. O host_world_dropin cria a sessão quando o **host abre o mundo**, sem match prévio. → Nova operação `OpenWorld(hostPlayerID, gameID)` que gera um `sessionID` próprio (o `matchID` deixa de ser a chave nesse arquétipo). **Recomendação:** manter as duas operações lado a lado (o `queue_arena` segue usando `Create`); a sessão ganha um campo `origin` (`match` | `host_world`) e `hostPlayerID` opcional.

2. **Quando alocar o dedicated server (custo — §6 da ficha).** No original o mundo é grátis (PC do host); para nós, alocar um Agones GameServer por jogador solo-online é proibitivo. Opções: **(a)** alocar já no `OpenWorld` (mundo sempre num server — simples, mais caro); **(b)** alocar **sob demanda** só quando o mundo "abre para multiplayer" (primeiro co-op pedido / primeira invasão casada) e liberar ao esvaziar (barato, exige handoff de estado). **Recomendação:** começar com **(a)** para validar o fluxo ponta a ponta e medir; deixar **(b)** como evolução declarada (é a economia real, mas introduz latência/handoff). Vira candidato a **ADR** quando escolhido.

3. **Resultado por evento ≠ outcome terminal (dona de H2).** `MatchOutcome`/`WinLossDraw` é um payload **único no fim**. O host_world_dropin reporta `ParticipationEvent{playerID, role, event, outcome}` **ao longo da vida** da sessão (`JOINED`/`BOSS_DEFEATED`/`HOST_KILLED`/`DIED`/`LEFT`). → Nova operação `ReportParticipationEvent` (streaming/incremental), **não** um novo `Kind` do `MatchOutcome` terminal. **Recomendação:** publicar cada evento no mesmo tópico de fan-out (`match-results`) com um **envelope discriminado por tipo** (`kind: participation_event`), retrocompatível — os consumers atuais ignoram o que não entendem, novos sinks reagem.

## Realidade cross-repo: o data plane não existe ainda

O `unity-host-world-dropin-data-plane` (Unity) é **⬜ planejado** ([REPO-TOPOLOGY.md](../REPO-TOPOLOGY.md)) — não há server que encarne o "mundo do host" nem emita `ParticipationEvent`. Consequência (igual ao princípio do `queue_arena`: **control plane primeiro**):

- Todo H0–H2 se **valida no control plane** com o adapter `static` do `ServerAllocator` (server mockado) e um **cliente de teste** simulando o server do mundo (chama `JoinWorld`/`ReportParticipationEvent` como o Unity chamaria). É o mesmo padrão que fechou o `queue_arena` antes do Unity.
- A validação **end-to-end real** (Agones + Unity encarnando o mundo, drop-in de verdade) é etapa **cross-repo posterior**, no novo repo de data plane. Este roadmap entrega o **contrato** que ela vai consumir — candidato a um `host_world_dropin-dataplane-contract.md` (par do que existe para o `queue_arena`) na fatia H2.

---

## H0 — Sessão host-cêntrica persistente (o pivô)

**Objetivo:** a sessão deixa de nascer só de um match. Um host **abre um mundo** (sessão persistente, dona por host), com **slots por papel** derivados do perfil, e phantoms **entram/saem no meio** com papel.

**Specs:** `session/011-host-world-session` (núcleo) + provável cauda `game-registry` só se precisar expor o perfil ao session (o session já lê perfil? confirmar no plan — senão, reusa o `ProfileProvider`/leitura de perfil que o matchmaking já faz).

**Escopo (session-service):**
- Nova entrada `OpenWorld(ctx, hostPlayerID, gameID) → (Session, ConnectGrant)`: lê o perfil `host_world_dropin`, deriva capacidade por papel (`CoopSlots`/`InvaderSlots`), aloca server (decisão #2 → começar por **(a)**, alocar aqui), emite connect token do host (papel `host`), registra sessão persistente. `ErrUnsupportedArchetype` se o jogo não for `host_world_dropin`.
- `JoinWorld(ctx, sessionID, playerID, role) → ConnectGrant`: ocupa um slot do papel se houver capacidade (`ErrRoleFull`); emite connect token com `role` no claim (o server lê no connection approval, como já faz com `team_id`). Papel válido = subconjunto de `EnabledRoles`.
- `LeaveWorld(ctx, sessionID, playerID)`: libera o slot; mundo segue aberto. Host sair/morrer → `CloseWorld` (encerra a participação de todos os phantoms).
- Modelo: `Session` ganha `Origin` (`match`|`host_world`), `HostPlayerID`, e slots por papel (`RoleSlots map[string]RoleSlot{Capacity, Occupants}`). Novo `CloseReason` `host_left`.
- Connect token: novo claim `role` (`host`|`coop`|`invader`), análogo ao `team_id` do `queue_arena` (ADR-0023).

**Depende de:** nada novo no domínio além do perfil (✅). **Tamanho:** **XL** — é o coração do arquétipo (reescreve o ciclo de vida da sessão). Candidata a quebrar em H0a (open/close + slots, sem role no token) e H0b (join/leave por papel + token com role) se ficar grande demais numa branch.
**Destrava:** H1 (matchmaking injeta chamando `JoinWorld`), H2 (eventos referenciam a sessão/papel).

---

## H1a — Matchmaking por injeção (invasão)

**Objetivo:** o **segundo modo** do matchmaking, o mais simples primeiro — **busca + injeção automática**. O invasor pede um alvo; o sistema encontra um mundo elegível e o **injeta** (o host não escolhe). Prova o modelo "descobrir sessão existente e entrar", não "formar partida nova".

**Specs (duas, acopladas — o lado session apareceu ao aterrissar no código):**
- **`session/012-invadable-world-directory`** (dependência): o `OpenWorld` da H0 passa a carregar os **atributos de matchmaking do host** (soul level, weapon tier) e o session expõe `ListInvadableWorlds(gameID)` — mundos abertos com **slot de invasor livre** + atributos. Fonte de dados **pura** (sem política de elegibilidade). Modelo **pull** (matchmaking consulta o session). Senha adiada (follow-up).
- **`matchmaking/006-world-discovery-invade`** (estrela): novo RPC unary `Invade(gameID, soulLevel, weaponTier)` → lê o perfil (`InvasionEnabled`/`LevelRangeEnabled`/`WeaponTierEnabled`), lista mundos via `session/012`, filtra por **faixa assimétrica** (matemática = função de domínio pura, testável), escolhe um e injeta via `JoinWorld` (H0) com `role=invader`. Retry no próximo mundo se encher na corrida (`ErrRoleFull`); `ErrNoEligibleWorld` se nada casar; `ErrInvasionNotSupported` se o arquétipo/flag não permite. Queue-form (`FindMatch`) intacto.

**Depende de:** H0 (`JoinWorld`). **Tamanho:** **L** (dois serviços). **Nota:** a senha (play-with-friends, que ignora a faixa) ficou como follow-up nas duas specs — H1a foca a invasão por faixa.

---

## H1b — Matchmaking por sinal (co-op, quadro de sinais)

**Objetivo:** o fluxo **assimétrico de dois passos** — o aliado **anuncia um sinal**, o host **vê os sinais compatíveis e escolhe** quem invocar. É um quadro de sinais, não uma fila que resolve sozinha.

**Specs:** `matchmaking/007-summon-signs`.

**Escopo (matchmaking-service):**
- `PostSummonSign(ctx, gameID, playerID, coopProfile) → signID`: o aliado deixa um sinal num pool (área/região) com sua faixa. TTL no sinal (some se não for invocado).
- `ListSigns(ctx, sessionID/hostFilter) → []Sign`: o host vê os sinais **compatíveis** (faixa + senha) com o seu mundo.
- `SummonSign(ctx, sessionID, signID) → MatchResult`: o host escolhe; o sinal vira injeção via `JoinWorld` com `role=coop`; token pro aliado. Sinal consumido.
- Store dos sinais: efêmero com TTL (Redis encaixa, como o leaderboard; decidir no plan). Idempotência: invocar um sinal já consumido = `ErrSignConsumed`.

**Depende de:** H0 (`JoinWorld` co-op) + H1a (reusa faixa/critério). **Tamanho:** **L** (dois passos + store de sinais + TTL).

---

## H2 — Resultado por evento/papel (fan-out) + contrato do data plane

**Objetivo:** o server do mundo reporta **eventos por participante** (não um placar terminal), o fan-out os distribui, e os sinks colhem por papel. Fecha o "gap central" do baseline para este arquétipo.

**Specs:** `session/013-participation-events` (+ consumers em achievements/analytics; doc `host_world_dropin-dataplane-contract.md`).

**Escopo:**
- **session:** `ReportParticipationEvent(ctx, sessionID, ParticipationEvent{playerID, role, event, outcome})` — `event ∈ {JOINED, BOSS_DEFEATED, HOST_KILLED, DIED, LEFT}`, `outcome ∈ {SUCCESS, FAIL, NEUTRAL}` interpretado pelo papel. Alguns eventos mexem no slot (`LEFT`/`DIED` do phantom liberam; `HOST_KILLED` encerra o mundo). Publica no `match-results` com envelope `kind: participation_event` (decisão #3).
- **achievements (consumer):** reage a eventos por papel — "venceu como invasor" (`HOST_KILLED`+`outcome=SUCCESS`), "ajudou N bosses como co-op" (`BOSS_DEFEATED` cumulativo). Idempotência por `(sessionID, playerID, event)` em vez de `matchID` (eventos múltiplos por sessão). **Reusa** o critério por-stat/evento da A1 do `queue_arena` onde der.
- **analytics (consumer):** agrega participações por papel/evento por jogo (invasões bem-sucedidas, co-ops concluídos). JSON aditivo.
- **leaderboard:** *opcional nesta fatia* — ranking de invasões/co-ops via `ZINCRBY` (reusa boards cumulativos da A2). Pode virar cauda.
- **Contrato:** `host_world_dropin-dataplane-contract.md` — o que o server do mundo Unity chama (`OpenWorld`? não, o host abre via control plane; server recebe injeções e reporta eventos), formato do `ParticipationEvent`, claim `role` no token. Par do `queue_arena-dataplane-contract.md`.

**Depende de:** H0 (sessão + papéis). **Tamanho:** **L–XL** (toca session + 2 consumers + contrato).

---

## H3 — Surfacing no console ✅ (2026-07-07)

**Objetivo:** o painel mostra o arquétipo host-cêntrico — mundos abertos com papéis/slots ocupados, sinais no quadro, participações por papel.

**Specs:** `admin-bff` + `admin-ui` (numeração seguinte livre).

**Escopo:**
- **Rooms:** a aba de salas ao vivo passa a distinguir `host_world` — mostra host, papéis, slots ocupados/capacidade (co-op X/2, invasor Y/1), em vez de "N/N players". Reusa o board de Rooms (session/007).
- **Analytics:** aba/coluna de participações por papel (invasões, co-ops) quando o jogo é `host_world_dropin`. Reusa o surfacing condicional por perfil da B2.
- (opcional) visualizar o quadro de sinais ativo de um mundo.

**Depende de:** H0 (Rooms lê slots), H2 (analytics lê participações). **Tamanho:** **M** (surfacing, reusa componentes).

---

## Resumo de sequência

| Fatia | Specs | Tamanho | Depende |
|---|---|---|---|
| H0 — sessão host-cêntrica ✅ **main** | session/011 | XL | perfil (✅) |
| H1a — injeção (invasão) | session/012 + matchmaking/006 | L | H0 |
| H1b — sinais (co-op) | matchmaking/007 | L | H0, H1a |
| H2 — evento/papel + contrato | session/013 (+consumers) | L–XL | H0 |
| H3 — console ✅ | admin-bff + admin-ui | M | H0, H2 |

Concluídas H0–H3, o `host_world_dropin` está **completo no control plane** (declarar → abrir mundo → descobrir/injetar por papel → reportar eventos → colher → console), validado com server mockado. Próxima etapa (cross-repo): o `unity-host-world-dropin-data-plane` (Unity) encarnando o mundo de verdade, consumindo o contrato de H2.

A senha (play-with-friends, adiada acima em H1a/H1b) foi resolvida depois — `session/020-password-world-matching` + `matchmaking/011-password-matching` (2026-07-09). Detalhe em [`host_world_dropin-follow-ups.md`](./host_world_dropin-follow-ups.md#1-play-with-friends-por-senha-o-mais-pedido---feito-2026-07-09).

## Aberturas que este roadmap **não** cobre (declaradas, adiadas)

- **Topologia `p2p`** (host como listen-server + relay/NAT): a ficha §"topologia" mostra que este arquétipo tem afinidade estrutural com P2P, mas é investimento (transport + TURN + repo Unity próprio). Fica como slot no perfil (`topology: p2p`), implementação = ADR futuro. Dedicated primeiro (é onde está a tese de autoridade neutra, ADR-0008).
- **Alocação sob demanda (decisão #2 opção b):** a economia real. Começar com alocação no `OpenWorld` e medir; migrar para handoff sob demanda quando justificar — candidato a ADR.
- **Covenants/facções, mensagens no chão (social assíncrono):** fora de escopo (ficha §"adjacência").
- **PvPvE simultâneo, "modo cinza" (Mound-maker):** papel não-binário; refina H0/H2 depois que o básico co-op+invasor rodar.
