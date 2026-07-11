# Spec: 005-team-composition

**Serviço:** matchmaking-service
**Status:** Done (2026-07-06)
**Data:** 2026-07-06
**ADR:** 0023 (perfil de multiplayer por jogo) — **fatia "composição de times"** (matchmaking atribui os lados)

## Contexto

A fatia 2 (spec `004-profile-driven-sizing`) fez o matchmaking **ler o perfil** do jogo e derivar apenas o **tamanho** do match (`ffa → playerCount`, `teams → teamCount × teamSize`), colapsando tudo num `int` (`ProfileProvider.MatchSize`). Ao formar, o match é entregue ao session como uma **lista plana** de jogadores (`Provision(matchID, players, gameID)`): não há noção de lados.

Só que o arquétipo `queue_arena` com `structure = "teams"` é **NvN** — os jogadores precisam ser **atribuídos a times**. Hoje ninguém faz essa atribuição: o `PlayerOutcome.TeamID` já existe no contrato (session spec `009`, proto), mas fica **vazio na prática**, porque nada popula os times na formação. Leaderboards/conquistas por time (fatias futuras) não têm como funcionar sem um `team_id` confiável.

Esta fatia faz o matchmaking **atribuir os jogadores a lados** quando o perfil é `teams`, e **propagar a composição** ao session no provisionamento. Fecha o "formar" da trilha do `queue_arena` (declarar → **formar** → produzir → colher). Escopo **restrito à composição**: sem skill/MMR, sem party, sem região, sem balanceamento por habilidade — atribuição posicional determinística (isso é backlog: "matchmaking com times e composição" no Log de generalizações, eixo *persistência competitiva*).

**Fronteira com o data plane (ADR-0010):** o matchmaking só **decide** os times e os entrega ao session (control plane). O session é o **dono canônico** da composição (ver spec `session/010-team-registration`); o game server (repo Unity) **não** precisa conhecer o `team_id` canônico — ele continua reportando o outcome por jogador, e o session carimba o time. Nada neste escopo toca o data plane.

## Comportamento

### Derivar a formação (não só o tamanho) do perfil

- WHEN o cliente chama `FindMatch{game_id}` (autenticado) → THE SYSTEM SHALL obter do perfil de multiplayer não apenas o tamanho, mas a **formação**: a `structure` (`ffa` | `teams`) e, quando `teams`, o `teamCount` e o `teamSize`.
- THE SYSTEM SHALL manter a derivação de tamanho da fatia 2 (`ffa → playerCount`; `teams → teamCount × teamSize`) e o critério de agrupamento FIFO por jogo (ADR-0017) **inalterados**.
- IF o jogo não tem perfil `queue_arena` utilizável (ausente, arquétipo errado, formação inválida, ou game-registry indisponível sem cache) → `FailedPrecondition`/`Unavailable` (**ErrProfileUnavailable**), o ticket **não** entra na fila — **inalterado** em relação à fatia 2.

### Atribuir os jogadores a times ao formar o match

- WHEN um grupo de `teamCount × teamSize` tickets do mesmo jogo é formado e a formação é `teams` → THE SYSTEM SHALL **particionar** os jogadores em `teamCount` times de exatamente `teamSize` jogadores cada, por **atribuição posicional determinística** (a ordem FIFO do grupo define os times; a regra exata — blocos contíguos vs. round-robin — é decisão do plan).
- THE SYSTEM SHALL rotular cada time com um **identificador estável dentro do match** (ex.: `"team-1"`, `"team-2"`; forma exata no plan), de modo que todo jogador do match receba exatamente um `teamID` e cada time tenha `teamSize` jogadores.
- WHEN a formação é `ffa` → THE SYSTEM SHALL formar o match **sem times** (composição vazia / `teamID` vazio por jogador) — comportamento equivalente ao atual.
- THE SYSTEM SHALL provisionar a sessão levando a **composição de times** ao session-service (a lista plana de jogadores deixa de ser suficiente para `teams`). A assinatura do `SessionProvisioner.Provision` muda para carregar os times (contrato abaixo).

### Invariantes preservadas

- Um ticket por jogador (re-`FindMatch` substitui o anterior) — inalterado.
- Cancelar o stream remove o ticket da fila — inalterado.
- Cada jogador recebe pelo seu canal o **próprio** connect token no sucesso, ou `MatchResult{Failed}` na falha — inalterado; o token não muda por causa dos times.
- Jogadores de jogos diferentes nunca entram no mesmo match — inalterado.
- Cache do perfil por `game_id` com TTL curto — mantido; passa a cachear a **formação** (tamanho + estrutura + teamCount/teamSize), não só o tamanho.

## Contratos de Interface

### Protobuf

`FindMatchRequest{ game_id }` **inalterado** (o cliente não escolhe time; a atribuição é do servidor). A mudança de contrato acontece no **gRPC do session** (`CreateSessionRequest` ganha a composição de times) — ver spec `session/010-team-registration`.

### Ports (domínio)

- **`ProfileProvider`** (Port de saída) passa a devolver a **formação**, não só o tamanho:
  - Substitui/estende `MatchSize(ctx, gameID) (int, error)` por algo como `Format(ctx, gameID) (MatchFormat, error)`, onde `MatchFormat{ Size int; Structure string; TeamCount int; TeamSize int }`. Forma exata (renomear vs. adicionar) é decisão do plan; a derivação `ffa`/`teams` continua morando no adapter (`registryclient`).
  - `ErrProfileUnavailable` em ausência/erro — inalterado.
- **`SessionProvisioner`** (Port de saída) passa a levar a composição de times:
  - `Provision(ctx, matchID string, teams []TeamAssignment, gameID string) (Provisioned, error)` — onde `TeamAssignment{ TeamID string; Players []string }`. Em `ffa`, um único "lado" sem `TeamID` (ou a lista plana representada como um time anônimo) — forma exata no plan, desde que o session receba tanto o conjunto de jogadores quanto o mapeamento jogador→time.
  - `Provisioned{ ServerAddress; Tokens map[playerID]token }` — **inalterado** (o token por jogador não depende do time).
- **`Matchmaker.Join`** — assinatura **inalterada**; passa a consultar o `ProfileProvider.Format`.

### Erros de domínio

- `ErrProfileUnavailable` — **inalterado** (agora também cobre formação `teams` com `teamCount`/`teamSize` inválidos vindos do perfil, que o game-registry já valida na escrita — spec `game-registry/006`; aqui é defesa em profundidade).

## Modelo de Dados

Sem persistência nova (fila em memória — spec 001). O ticket passa a carregar a **formação** derivada (estrutura + teamCount/teamSize), não só `matchSize`. A composição de times é calculada na formação e vive apenas no fluxo de provisionamento (não é persistida no matchmaking; o dono é o session — spec `session/010`).

## Fora de escopo (backlog explícito)

- **Balanceamento por skill/MMR** entre os times (hoje: atribuição posicional). — Log de generalizações, "persistência competitiva".
- **Party/pré-grupos** (amigos no mesmo time) e **preferência de time**.
- **Squads de tamanho variável** e **lobby grande** (arquétipo `battle_royale`).
- **Colher por time** (leaderboard/conquistas/analytics por `teamID`) — fatias seguintes; esta só garante o `teamID` confiável na origem.

## Critérios de Aceite

- [ ] `teams`: um grupo de `teamCount × teamSize` jogadores é particionado em `teamCount` times de `teamSize`; todo jogador tem exatamente um `teamID`; times têm rótulo estável no match.
- [ ] `ffa`: match formado sem times (comportamento da fatia 2 preservado); `teamID` vazio.
- [ ] O session recebe a composição (jogador→time) no provisionamento; a lista de jogadores continua completa.
- [ ] Perfil ausente/errado/indisponível → `ErrProfileUnavailable`, ticket fora da fila (inalterado).
- [ ] Métricas de fila/matches formados/falhos preservadas.
- [ ] Testes unitários do particionamento (tamanhos 2×1, 2×2, 3×3, ffa) com `-race`; cobertura `service/` ≥ 80%.
