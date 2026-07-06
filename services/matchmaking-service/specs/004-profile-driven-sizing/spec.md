# Spec: 004-profile-driven-sizing

**Serviço:** matchmaking-service
**Status:** Implementada (2026-07-06)
**Data:** 2026-07-06
**ADR:** 0023 (perfil de multiplayer por jogo) — **fatia 2** (matchmaking lê o perfil)

## Contexto

Segunda fatia do ADR-0023. Hoje o matchmaking forma matches com um **`matchSize` global fixo** (injetado no boot, igual para todos os jogos — `service/matchmaker.go`), agrupando tickets por `game_id` em FIFO (spec 003-game-scope). Com o perfil de multiplayer no game-registry (fatia 1, arquétipo `queue_arena`), o **tamanho da partida passa a ser uma propriedade do jogo**, não uma constante global.

Esta fatia faz o matchmaking **ler o perfil do jogo** (do game-registry) e **derivar o tamanho do match** dele: `playerCount` (estrutura `ffa`) ou `teamCount × teamSize` (estrutura `teams`). É o primeiro serviço a **consumir** o perfil — materializa o "serviços leem o perfil e se configuram" do ADR-0023.

Escopo restrito ao **tamanho** (e validação da estrutura). A **composição/atribuição de times** e o **resultado polimórfico** ficam para a fatia 3 (session), porque só têm efeito com a session/data plane cientes dos times — evita trabalho morto aqui.

## Comportamento

### Ler o perfil e derivar o tamanho ao entrar na fila

- WHEN o cliente chama `FindMatch{game_id}` (autenticado) → THE SYSTEM SHALL obter o **perfil de multiplayer** do jogo (via game-registry) e **derivar o tamanho do match**:
  - estrutura `ffa` → `playerCount`;
  - estrutura `teams` → `teamCount × teamSize`.
  Em seguida enfileira o ticket, associando-o a esse tamanho.
- THE SYSTEM SHALL formar matches por jogo usando o **tamanho derivado do perfil daquele jogo** (não mais um `matchSize` global). Jogadores de jogos diferentes seguem nunca entrando no mesmo match (ADR-0017, inalterado).
- IF o jogo **não tem perfil** configurado, ou o perfil **não é `queue_arena`** → `FailedPrecondition` (**ErrProfileUnavailable**); o ticket **não** entra na fila.
- IF o game-registry estiver **indisponível** e não houver perfil em cache → `Unavailable` (**ErrProfileUnavailable**); o ticket não entra.

### Cache do perfil por jogo

- THE SYSTEM SHALL **cachear** o tamanho derivado por `game_id` com um **TTL curto** (evita uma chamada ao game-registry por ticket). Um cache-hit válido dispensa a chamada.
- THE SYSTEM SHALL tolerar mudança de perfil eventualmente (o TTL delimita a janela de defasagem — aceitável no MVP; matches já formados não são afetados).

### Invariantes preservadas

- Um ticket por jogador (re-`FindMatch` substitui o anterior) — inalterado.
- Cancelar o stream remove o ticket — inalterado.
- Ao formar, provisiona a sessão via `SessionProvisioner.Provision(matchID, players, gameID)` — **assinatura inalterada** nesta fatia (times entram na fatia 3).

## Contratos de Interface

### Protobuf

Sem mudança: `FindMatchRequest{ game_id }` (spec 003) permanece. O tamanho deixa de ser configuração do serviço e passa a ser derivado do perfil.

### Ports (domínio)

- **Novo** `ProfileProvider` (Port de saída) — lê o perfil do jogo e devolve o tamanho do match:
  - `MatchSize(ctx, gameID string) (int, error)` — devolve o tamanho derivado do perfil `queue_arena`; `ErrProfileUnavailable` se ausente/errado/indisponível.
  - (A derivação `ffa`/`teams` → tamanho mora no adapter que fala com o game-registry; o domínio só recebe o tamanho.)
- `Matchmaker.Join(ctx, gameID, playerID)` — **assinatura inalterada**; passa a **consultar o `ProfileProvider`** e falhar com `ErrProfileUnavailable` quando não conseguir o tamanho.
- `SessionProvisioner.Provision(...)` — **inalterado**.

### Erros de domínio

- **Novo** `ErrProfileUnavailable` — jogo sem perfil `queue_arena` utilizável (ausente, arquétipo errado, ou game-registry indisponível sem cache). Mapeado no transport para `FailedPrecondition` (sem perfil/arquétipo) — a distinção fina de `Unavailable` é decisão do plan.

## Modelo de Dados

Sem persistência nova no matchmaking (fila é em memória — spec 001). O cache do perfil é **em memória** com TTL (estrutura e TTL definidos no plan). O perfil canônico vive no game-registry (fatia 1).

## Casos de Teste Obrigatórios

- [ ] (service) `Join` de jogo com perfil `ffa playerCount=2` → forma match ao juntar 2 tickets do jogo
- [ ] (service) `Join` de jogo com perfil `teams teamCount=2 teamSize=2` → forma match com 4 tickets
- [ ] (service) dois jogos com tamanhos diferentes formam matches independentes, cada um no seu tamanho
- [ ] (service) jogo sem perfil / arquétipo ≠ queue_arena → `Join` retorna `ErrProfileUnavailable`; ticket não entra na fila
- [ ] (service) `ProfileProvider` indisponível sem cache → `ErrProfileUnavailable`
- [ ] (service) cache: N `Join` do mesmo jogo dentro do TTL → uma chamada ao provider (mock conta as chamadas)
- [ ] (transport/gRPC) `ErrProfileUnavailable` → `FailedPrecondition`
- [ ] (adapter, se aplicável) derivação: perfil `ffa`→playerCount; `teams`→teamCount×teamSize

## Fora do escopo

- **Composição/atribuição de times** (quem joga em qual lado) — fatia 3 (session + resultado).
- **Resultado polimórfico** — fatia 3.
- **Skill/MMR, região, modo/party** — backlog (ADR-0023 não os cobre nesta fatia).
- **Invalidação ativa do cache** ao mudar o perfil — só TTL no MVP.
- **Mudança do contrato `Provision`** (continua `players []string`).
- **Outros arquétipos** (`battle_royale`, etc.) no matchmaking — quando existirem no game-registry.
