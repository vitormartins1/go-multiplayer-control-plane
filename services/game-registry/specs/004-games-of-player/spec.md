# Spec: 004-games-of-player

**Serviço:** game-registry (consulta reversa) + admin-bff + admin-ui
**Status:** Implementada (2026-06-30)
**Data:** 2026-06-30
**Relaciona:** ADR-0019 (presença por jogo), admin-ui/005 (jogo ativo global)

## Contexto

Refina o fluxo jogo-cêntrico para ficar natural com a **identidade global** (ADR-0019). Hoje o painel mistura três conceitos; esta feature separa:
1. **Base global** — todos os que têm conta (identity, já existe `/api/players`).
2. **Presentes no jogo** — quem entrou no jogo X (presença, `game_players`).
3. **Jogos de um player** — consulta **reversa** da presença (novo).

Resolve dois incômodos: (a) no detalhe do player, o seletor de jogo deve listar **só os jogos do player** (pré-selecionando o primeiro), não todos; (b) a aba Players deve mostrar **toda a base** por padrão (sem exigir escolher jogo), com um modo opcional "presentes no jogo".

> "Presença" = entrou no jogo ao menos uma vez (`first_seen`), **não** "online agora".

## Comportamento

### game-registry (consulta reversa — control plane interno)
- WHEN `GET /players/{playerID}/games` → lista os **jogos em que o player está presente** (JOIN `game_players` × `games`), por nome. Vazio se não entrou em nenhum.

### admin-bff
- WHEN `GET /api/players/{playerID}/games` → repassa os jogos do player. 502 se o game-registry cair.

### admin-ui — navegação player-cêntrica (decisão deste review)
A confusão vinha do **jogo ativo global no header** (admin-ui/005), que impunha um único jogo a tudo e colidia com player ∈ 0..N jogos. Decisão: **remover o seletor global** e dar a cada seção o seu escopo (dois eixos):
- **Games** (game-first): biblioteca → abre um jogo → catálogo, leaderboard e **aba Players** = base daquele jogo (presença). É o "qual player base de qual jogo".
- **Players** (global): base de usuários (`api.players`, identity), independente de jogo, com avatares. Abrir um player → detalhe.
- **PlayerDetail:** seletor escopado aos **jogos do player** (`api.gamesOfPlayer`), default = primeiro; abas usam esse jogo. Sem jogos → "ainda não entrou em nenhum jogo".
- **Header:** sem seletor de jogo (vestigial neste modelo). `GameProvider`/`useGames`/`GameSwitcher` removidos.

## Contratos
- game-registry: `PresenceRepository.GamesByPlayer(ctx, playerID) []Game`; `PresenceService.GamesOf(...)`; rota `GET /players/{playerID}/games`; guard passa a cobrir `/players` além de `/games`.
- admin-bff: `GameRegistry.GamesOfPlayer(ctx, playerID) []Game`; adapter; `GET /api/players/{playerID}/games`.
- admin-ui: `api.gamesOfPlayer(playerId)`; `PlayerDetail` (seletor local); `Players` (SegmentedControl Todos/Por jogo).

## Casos de Teste Obrigatórios
- [x] (game-registry) GamesByPlayer só os jogos do player; vazio se nenhum — integração
- [x] (game-registry) `/players/...` exige internal key
- [x] (admin-bff) `/api/players/{id}/games` 200 + 502
- [x] (ui) PlayerDetail lista só os jogos do player (default primeiro); Players global; base por jogo na aba de GameDetail; build ok

## Fora do escopo
- "Online agora" (sessão ativa) — só presença (entrou ao menos uma vez)
- Paginação da presença / busca — incremental
