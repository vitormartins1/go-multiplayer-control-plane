# Sinais do Folclore Souls — o que o jogo sabe e o que ele pede ao control plane

> Escrito em 2026-09-26 a partir do repositório do jogo (`unity-host-world-dropin-data-plane`, que NÃO é
> visível numa sessão de nuvem deste repo). Serve de evidência para a revisão de produto
> (`docs/prompts/PROMPT-REVISAO-PRODUTO.md`). Cada item diz a fonte no repo do jogo; trate como relato,
> e confirme no código DESTE repo antes de agir.

## O jogo, em uma frase

**Folclore Souls** (nome legado *Capoeira Souls*, rename técnico feito em 2026-08-19 no jogo e 2026-08-24 aqui;
o `gameId` do Unity é o UUID e nunca mudou): um Souls-like de folclore brasileiro em Unity 6 (NGO, servidor
dedicado autoritativo, Agones). O multiplayer é o arquétipo **`host_world_dropin`** deste repo: o host abre o
mundo, co-op entra por sinais, invasor entra pelo `Invade`, o servidor reporta **eventos por participante**
(`JOINED/BOSS_DEFEATED/HOST_KILLED/DIED/LEFT`), sem placar. Contrato canônico:
`docs/game-models/host_world_dropin-dataplane-contract.md`.

## Onde o jogo está (2026-09)

- **O contrato inteiro já foi exercitado de ponta a ponta** em julho (specs 001-004 do jogo): `OpenWorld` + token
  EdDSA validado offline via JWKS, `JoinWorld`, `Invade`, quadro de sinais (`PostSummonSign`/`ListSigns`/
  `SummonSign`), senha/play-with-friends, `ReportParticipationEvent` com `event_id` idempotente, manifest
  (`GetSessionConfig` → `params_json`), bateria real no Agones (k3d) em 2026-07-12.
- **Desde agosto o foco é GAMEPLAY**, não multiplayer (decisão do dono, 2026-09-22): bosses de arena (Boitatá,
  Cuca, Lobisomem, Mapinguari), combate estilo FromSoftware, mundo aberto. *"A gente ainda não está trabalhando
  nas mecânicas de co-op, invasor, mecânica multiplayer. O nosso foco agora é mecânica de gameplay."*
- Consequência: vários achados de multiplayer no jogo estão **adiados** até a fase de multiplayer voltar — o
  backend não precisa correr na frente deles, mas pode se preparar onde o desenho já está claro.

## O que o jogo já pediu ou vai pedir ao backend (fonte: `docs/backlog/netcode-e-backend.md` do jogo)

Contrato e sessão:
- **Allocator fleet-aware por `game_id`** — ADR-0026 (Proposed) DESTE repo, não implementado. Sem isso, dois jogos
  no mesmo cluster Agones alocam da fleet errada (hoje `AGONES_FLEET` fixo). Pré-requisito da fase de deploy real.
- **Alocação sob demanda** (follow-up #10 do arquétipo): hoje aloca no `OpenWorld`; o original (Dark Souls) é P2P e
  o mundo "existe de graça" — com servidor dedicado, o custo pede alocar só quando o mundo abre para multiplayer.
- **Exigir bearer no `OpenWorld` e derivar o host do bearer** — o jogo ainda extrai o `sub` do JWT localmente porque
  o backend não exige o bearer (`SessionRpcClient.cs`, `JwtPayload.cs` do jogo).
- **Reconexão do host e de phantom** — nunca previstas no contrato.
- **Coops no `players[]` da sessão** — achado ao vivo (2026-07-12): o invasor aparecia na sala do admin, os coops
  via `SummonSign` não (conferir se já foi resolvido pela `session/025`).
- **Identidade do boss no `BOSS_DEFEATED`** — hoje o evento é anônimo; o jogo tem vários bosses e quer
  achievements/analytics por boss (o lore do Boitatá já pede isso).
- **Fila de espera de invasão** (sem retry automático hoje) e **pools por região** (`areaPools`, follow-up #2).
- **9 params novos propostos para o arquétipo** (`invasionRequiresCoop`, `hostDeathPolicy`,
  `invasionCooldownSeconds`, …) — só por evidência de modo real (`docs/MODES.md` do jogo).
- **`stats` no `ReportParticipationEvent`** — o backend aceita desde 2026-07-13; o jogo ainda não preenche.
- **Loadout de sessão vindo do control plane** (equipamento por sessão, ADR-0014 do jogo) — gancho previsto.
- **Progresso do mundo** (checkpoint, bosses derrotados) vive no client hoje; decisão em aberto se vai para o
  cloud-save/player-data.

Operação (bugs reais que o jogo achou aqui):
- **2026-08-19, resolvido**: `LEFT/JOINED` refém do publish de analytics (localstack morto 3 dias + o session-service
  publicava ANTES de mutar). Consertado com mutação idempotente antes do publish. Lição: gameplay nunca pode ser
  refém de analytics; serviços do compose sem restart policy.
- **Histórico de sessões in-memory** → resolvido pela `session/025` (Postgres).

Decisões em aberto registradas no jogo:
- `gameId` obrigatório no login? (addendum do ADR-0019 deste repo, não decidido)
- As 3 "aberturas maiores" do roadmap do arquétipo: topologia P2P, PvPvE "modo cinza", covenants/mensagens
  assíncronas (as mensagens no chão e as manchas de sangue do Dark Souls são o "assíncrono" que o jogo mais
  naturalmente usaria).
- R&D de topologia netcode DS/ER-like (offline solo + co-op P2P) no jogo (ADR-0002/0010 do jogo).

## O que a experiência do jogo ensinou sobre "genérico"

- O jogo validou o `host_world_dropin` porque ele foi desenhado a partir de um caso real (ficha
  `01-soulslike-invasion-coop`). `battle_royale` e `coop_mission` têm contrato pronto e **nenhum jogo real** — o
  que é genérico sem caso real tende a ser especulação.
- O que realmente foi usado pelo jogo: identity (login anônimo e registrado), game-registry (modos + manifest),
  matchmaking (invasão/sinais/senha), session (mundo, eventos, JWKS), achievements (event-rules data-driven,
  ADR-0027), analytics. **Pouco ou nada exercitado pelo jogo até hoje**: leaderboard, cloud-save, player-data
  (o progresso do mundo ainda é local), battle_royale/coop_mission.
- O dono sente que "viajou" em partes do admin/dashboard (ex.: dashboard configurável por widget). Isso é um
  sinal, não um veredito: a revisão deve medir o que cada peça entrega a quem opera um jogo real.
