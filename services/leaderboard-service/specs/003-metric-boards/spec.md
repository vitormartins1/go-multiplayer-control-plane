# Spec: 003-metric-boards

**Serviço:** leaderboard-service (+ consumidor do fan-out)
**Status:** Implementada (2026-07-06)
**Data:** 2026-07-06
**ADR:** 0023 (resultado polimórfico) — **fatia 3, consumidor**; relaciona ADR-0020 (fan-out)

## Contexto

A fatia 3 (session `009`) enriqueceu o resultado com **stats** por jogador (`kills`, `assists`, …), além de `score`. Hoje o leaderboard mantém **um único ranking por jogo** (highscore de `score`) e ignora o resto. Esta fatia generaliza o leaderboard para **N rankings por jogo, um por métrica** (`board`): o `score` de sempre **mais** um board por stat presente no resultado. Mesma semântica **highscore** (o melhor valor numa partida).

É a colheita do resultado rico no leaderboard (par do achievements `007`). É **control plane puro** — não depende do data plane. Fica **fora** semântica **acumulativa** (total de vitórias/kills), **razões** (K/D), e boards declarados/validados pelo perfil.

## Comportamento

### Rankings por métrica (board)

- THE SYSTEM SHALL identificar cada ranking por `(gameID, board)`, onde `board` é `"score"` (default) ou o nome de um stat (ex.: `"kills"`). Cada board é um highscore independente.
- WHEN o fan-out entrega um resultado → THE SYSTEM SHALL submeter, por jogador: o `score` ao board **`score`** e **cada `stat`** ao board de nome igual à métrica (ex.: `stats.kills` → board `kills`).
- THE SYSTEM SHALL aplicar **highscore por board** (só sobe se o novo valor for maior — `ZADD GT`), como hoje, independente entre boards.
- IF o resultado não tiver `stats` (envelope antigo/v1, ou sem stats) → THE SYSTEM SHALL submeter só o `score` ao board `score` (comportamento atual — retrocompat).
- THE SYSTEM SHALL aceitar como nome de board os identificadores estáveis já validados no perfil (`^[a-z][a-z0-9_]*$`); nomes fora do padrão são ignorados (não derrubam o processamento).

### Leitura por board

- WHEN `GET /leaderboard?board=<nome>` (e `?limit`) → THE SYSTEM SHALL devolver o Top do board pedido; **`board` ausente → `score`** (retrocompat da API atual).
- WHEN `GET /leaderboard/me?board=<nome>` (e `?around`) → idem para a posição do jogador.
- O admin (`/admin/games/{gameId}/leaderboard`) SHALL aceitar `?board` (default `score`).

## Contratos de Interface

- **Domain** (`LeaderboardService`/`LeaderboardStore`): os métodos (`Submit`, `Top`, `Around`/`Range`, `Rank`, `ForceSubmit`/`ForceScore`) ganham o parâmetro **`board string`**. `Entry` inalterado.
- **Storage** (Redis): a key passa de `<prefix>:<gameID>` para **`<prefix>:<gameID>:<board>`**. (Reset dos rankings de dev — Redis efêmero, aceitável.)
- **Consumer** (`cmd/consumer/main.go`): o envelope ganha `stats map[string]int64` por resultado; submete `score` + cada stat ao seu board.
- **Transport HTTP:** `board` como query param opcional (default `score`) nas rotas de leitura; validação do nome no service.

## Casos de Teste Obrigatórios

- [ ] (storage/integração) Submit em boards distintos do mesmo jogo são **independentes** (`kills` não afeta `score`); highscore por board (ZADD GT)
- [ ] (service) `board` inválido (fora do regex) → erro de validação / ignorado no fan-out
- [ ] (consumer) resultado com `stats` → submete `score` ao `score` e cada stat ao seu board; sem `stats` → só `score` (retrocompat)
- [ ] (transport) `GET /leaderboard?board=kills` → Top do board kills; **sem `board` → score** (compat)
- [ ] (transport) `/leaderboard/me?board=…` e admin `?board` respeitam o default `score`
- [ ] Top/Around de board inexistente → lista vazia (não erro)

## Fora do escopo

- **Leaderboards acumulativos** (total de vitórias/kills ao longo de partidas) — semântica diferente (ZINCRBY); fatia própria.
- **Razões** (K/D) e métricas derivadas — evolução.
- **`outcome`/`teamId`** no leaderboard (ranking de vitórias, por time) — evolução.
- **Declarar/validar os boards pelo perfil** (`scoringStats` do game-registry) — aqui os boards existem implicitamente pelos stats que chegam; validar contra o perfil é evolução.
- **analytics** consumindo os campos ricos — spec própria.
