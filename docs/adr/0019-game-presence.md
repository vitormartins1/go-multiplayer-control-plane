# ADR-0019: Presença por jogo (player base por jogo) sobre identidade global

**Status:** Accepted
**Data:** 2026-06-30
**Serviço(s) afetado(s):** game-registry, identity-service, admin-bff, admin-ui
**Relaciona / refina:** ADR-0017 (multi-tenant por `game_id`, identidade global); ADR-0009 (identity como fundação); ADR-0018 (auth serviço-a-serviço)

---

## Contexto

O ADR-0017 (#2) decidiu **identidade global**: uma conta serve vários jogos e "quais jogos um user joga **emerge dos dados** por-jogo dele". Isso mantém o jogador único (não duplica), mas tem um custo concreto que aparece no painel: **listar "a player base do jogo X" é caro e impreciso** — exigiria agregar quem tem dado em 4 serviços (achievements/player-data/cloud-save/leaderboard), e quem se registrou mas ainda não gerou dado não apareceria.

Com o painel agora **jogo-cêntrico** (admin-ui/005: jogo ativo global + biblioteca de capas), a navegação natural é **jogo → players do jogo → dados do jogador**. Falta a peça "players do jogo".

A tentação seria tornar a **conta** por-jogo (a "Opção D" do 0017), mas isso duplica o jogador e quebra o caso "mesma pessoa em vários jogos" (modelo launcher/SSO, tipo Steam/Epic). Queremos o melhor dos dois: conta global **e** uma player base por jogo precisa.

## Alternativas Consideradas

### Opção A: Presença explícita no game-registry (escolhida)
Uma associação `(game_id, player_id, handle, first_seen)` — o jogador "está presente" num jogo. Mora no **game-registry** (já é a autoridade de tenant/jogo; passa a guardar `player_id` + `handle` desnormalizado, **sem credenciais**). Marcada no **login "dentro do jogo"**.
- **Prós:** player base por jogo precisa e barata (uma query); identidade global intacta (0017 #2 preservado); não duplica o jogador; casa com "logar de dentro do jogo"; o tenant authority (game-registry) é o dono natural da associação.
- **Contras:** acopla game-registry a `player_id`/`handle` (leve, sem credenciais); o `handle` é desnormalizado (pode ficar defasado se mudar — aceitável no MVP); presença é **best-effort** (um login sem `game_id` não marca).

### Opção B: Derivar a base dos dados por-jogo
Agregar quem tem achievement/data/save/score no jogo.
- **Contras:** cara (fan-in de 4 serviços), imprecisa (quem não gerou dado some), sem um dono claro. **Rejeitada.**

### Opção C: Conta por jogo (`game_id` em `accounts`) — a "Opção D" do 0017
- **Contras:** duplica o jogador; perde "mesma pessoa em vários jogos"; mexe pesado no identity (login por jogo, JWT com game_id); supersede 0017/0009. **Rejeitada.**

### Opção D: Presença no identity-service
- **Contras:** o 0017 manteve o identity como "o jogador", global e sem `game_id`; meter a dimensão de jogo lá contraria isso. O game-registry já é multi-tenant-aware. **Preterida** em favor de A.

## Decisão

1. **Identidade continua global** (ADR-0017 #2 e ADR-0009 inalterados): conta única, sem `game_id` em `accounts`.
2. **Presença explícita** é uma nova dimensão no **game-registry**: tabela `game_players (game_id, player_id, handle, first_seen)`, PK `(game_id, player_id)`, FK `game_id → games(id)` (mesmo banco, sem FK cross-service). `handle` é **desnormalizado** (vem do login) para o painel listar sem um lookup no identity.
3. **game-registry** expõe (control plane interno, atrás da internal key — ADR-0018, prefixo `/games`):
   - `PUT /games/{gameId}/players/{playerID}` — upsert de presença (`first_seen` no primeiro; idempotente).
   - `GET /games/{gameId}/players` — a player base do jogo.
4. **Marcação no login "dentro do jogo":** o identity passa a aceitar um `gameId` **opcional** no corpo de `POST /sessions` e `POST /sessions/anonymous`. Quando presente, após autenticar, o identity marca presença no game-registry — **best-effort** (loga falha, não derruba o login), no espírito do fan-out do ReportResult. Isso torna o identity um caller serviço-a-serviço (injeta `X-Internal-Key`, ADR-0018).
5. **Painel jogo-cêntrico:** a aba **Players** passa a listar a base do **jogo ativo** (via admin-bff → game-registry); navegação jogo → players → dados, coerente com o contexto global do header (admin-ui/005).

## Consequências

### Positivas
- Player base por jogo **precisa e barata** (uma query no game-registry); painel jogo-cêntrico fechado.
- Identidade global preservada (não duplica jogador; "mesma pessoa em vários jogos" continua valendo).
- Modelo **genérico**: serve tanto o cenário launcher/SSO quanto o multi-tenant isolado.

### Negativas / Trade-offs
- game-registry passa a conhecer `player_id` + `handle` (acoplamento leve; **nunca** credenciais).
- `handle` desnormalizado pode defasar se o usuário trocar de handle (aceitável; reconciliação fica como evolução).
- Presença é **best-effort**: um login que não informa `game_id` não marca; players antigos só aparecem na base do jogo após o próximo login-into-game.

### Neutras / Implicações
- O identity ganha uma dependência de saída para o game-registry (mesma internal key do resto).
- Decorre a feature SDD `game-registry/002-game-presence` (núcleo + integração identity/admin-bff/admin-ui).
- A capa real do jogo (`coverUrl`) é outra evolução do game-registry (feature à parte).

## Referências
- ADR-0017 (multi-tenant, identidade global), ADR-0009 (identity fundação), ADR-0018 (internal key)
- admin-ui/005-game-library (painel jogo-cêntrico), services/game-registry/specs/002-game-presence

## Addendum (2026-07-09): trade-off observado na prática — revisitar ou não?

Durante a verificação da spec 004 do `unity-host-world-dropin-data-plane`, o trade-off da linha 55 ("presença é best-effort... players antigos só aparecem na base do jogo após o próximo login-into-game") apareceu **na prática, não como hipótese**: `IdentityClient.LoginAsync`/`LoginAnonymousAsync` desse data plane **nunca mandavam `gameId`** — nenhum jogador (guest ou registrado) tinha presença marcada. Sintoma reportado pelo usuário: um achievement já progredindo de verdade (confirmado via `GET /admin/games/{id}/players/{playerID}/achievements`) **não aparecia na página do próprio jogador** no admin-ui, porque `PlayerDetail.tsx` decide quais jogos oferecer via `GamesOfPlayer` (mesma tabela `game_players`) — sem presença, o jogo nem aparecia como opção selecionável, mascarando dado que já existia. Corrigido no data plane (`IdentityClient` passou a mandar `_config.GameId`); presença retroativa de contas de teste já usadas foi backfilled com um novo login manual (mesmo endpoint, sem mudança de schema).

**Achado ao checar o repo irmão:** `unity-queue-arena-data-plane` tem a **mesma lacuna** — seu `IdentityClient` também nunca manda `gameId`. Não é um padrão estabelecido que um repo seguiu e o outro não; é uma lacuna nos dois data planes Unity existentes (o parâmetro só existe desde este ADR, 2026-06-30, e nenhum dos dois foi retrofitado até agora). **Follow-up:** aplicar o mesmo fix no `unity-queue-arena-data-plane`.

**Pergunta em aberto, levantada pelo usuário (não decidida aqui):** já que o sistema pede login "dentro do jogo" pra presença fazer sentido, faria mais sentido **tornar `gameId` obrigatório** no login (convenção de todos os data planes) e o achievements-service **validar presença antes de registrar progresso** (hoje ele grava progresso só com o `playerID` do JWT — `services/achievements-service` não tem nenhuma lógica de presença, checada e confirmada ausente)? Isso **reabriria a Opção C rejeitada acima** (linha 29-30) na prática, ainda que sem duplicar a conta: um client tipo launcher que ainda não escolheu o jogo não teria como logar sem escolher um `gameId` primeiro. **Não adotado agora** — registrado aqui como candidato a reavaliação futura, por evidência (ex.: se aparecer um caso real de achievement "fantasma" por presença nunca marcada e não corrigível por um novo login, ou se o produto decidir que login sempre é "dentro de um jogo" e nunca um launcher multi-jogo).
