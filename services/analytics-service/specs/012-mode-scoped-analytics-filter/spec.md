# Spec: 012-mode-scoped-analytics-filter

**Serviço:** analytics-service (fatia em admin-bff)
**Status:** Done
**Data:** 2026-07-13
**Parte de:** evolução do Analytics (item A da ordem A→B→C→D decidida com o
usuário) — implementa o seletor de modo que as specs 010/011 deixaram
explicitamente pendente ("soma todos os modos, sem seletor nesta v1").

## Contexto

`GameAnalytics` (porta de saída do `MatchRepository`) sempre agrega
`PlacementDaily`/`MissionDaily`/`ParticipationDaily`/`Stats` **somando todos
os modos do jogo**, sem forma de isolar um modo específico — decisão
deliberada das specs 010/011, documentada no código (`postgres_analytics.go`:
"soma todos os modos (spec 010)") mas nunca revisitada.

Efeito prático, confirmado no código: se dois modos do MESMO jogo reportam
uma stat com a mesma chave (ex.: `kills` num modo `queue_arena` e `kills` num
modo `host_world_dropin` do mesmo jogo), a query de `Stats`
(`GROUP BY stat_key`, sem `mode_id`) soma os dois num único número — dois
sinais de proveniência diferente viram um só, sem como desfazer no cliente
(a query não devolve o dado por modo, só o agregado final). Mesmo problema
estrutural nas 3 séries diárias novas da spec 010 (todas com `mode_id` na
tabela, nenhuma expondo o filtro).

Isso foi apontado pelo usuário como parte do "dado bagunçado" do Analytics —
confirmado como causa raiz nesta revisão, não só percepção de UI.

**Achado adicional durante a implementação, trazido pro escopo a pedido do
usuário:** `domain.MatchResult` (caminho `queue_arena`/`win_loss_draw`) NÃO
tem campo `ModeID` — diferente dos irmãos `PlacementResult`/
`CollectiveResult`/`ParticipationEvent`, que ganharam o campo na spec 010.
O envelope do session-service já publica `modeId` pra esse kind
(`sns_publisher.go`), mas `consumer.parse()` nunca extrai, e
`ApplyMatch` grava `insertPlayerStats(ctx, tx, m.GameID, "", m.MatchID,
...)` — `""` HARDCODED, nunca `m.ModeID`. Efeito: **`queue_arena` (o
arquétipo mais comum) nunca captura `mode_id` em `game_player_stats`** —
todas as stats caem sempre no bucket vazio, e o filtro `?modeId=` desta
spec seria inútil pra esse arquétipo (filtrar por um modo real devolveria
vazio mesmo havendo dado). Mesmo padrão dos gaps já fechados nesta sessão
(campo existe nos "primos" mas não em `MatchResult`).

## Comportamento

- `GameAnalytics` (porta e todas as implementações) SHALL aceitar um
  `modeID string` opcional. `modeID == ""` (default, compatível com o
  comportamento atual) -> soma todos os modos, EXATAMENTE como hoje —
  nenhuma regressão pra quem já chama sem o parâmetro.
- `modeID != ""` -> `PlacementDaily`, `MissionDaily`, `ParticipationDaily`,
  `Stats` e `RoleParticipations` SHALL ser filtrados pra esse modo (`WHERE
  ... AND mode_id = $modeID`, nas tabelas que já têm a coluna desde as specs
  009/010/011).
- Os campos de `GameStat` embutido (`Matches`, `Participations`, `Wins`,
  `Losses`, `Draws`, `AvgScore`, `UniquePlayers`) e `Placement`/`Mission`
  (distribuição cumulativa) SHALL continuar SEM filtro por modo, mesmo com
  `modeID` != "" — as tabelas de onde vêm (`game_rollup`,
  `game_placement_rollup`, `game_mission_rollup`) não têm a coluna
  `mode_id` (limitação conhecida, item D da mesma revisão — fora de escopo
  aqui). O DTO SHALL deixar isso rastreável (ver Contratos).
- `GET /admin/games/{gameId}/analytics` (analytics-service) SHALL aceitar
  `?modeId=` opcional, repassado direto pra `GameAnalytics`. Query inválida
  (modo que não existe no jogo) não é erro — simplesmente devolve séries/
  stats vazias pra esse modo (mesmo espírito de "sem dado ainda" já usado
  em todo o resto do endpoint).
- **Sem mudança de UI nesta spec** — o admin-ui continua chamando sem
  `modeId` (item B, próximo da ordem combinada, é quem desenha o seletor e
  passa a usar o parâmetro).

### ModeID em MatchResult (queue_arena/win_loss_draw)

- WHEN o consumer processa um evento `kind=win_loss_draw` -> THE SYSTEM
  SHALL extrair `modeId` do envelope pra `domain.MatchResult.ModeID` (campo
  novo), mesmo padrão já usado em `parsePlacement`/`parseCollective`/
  `parseParticipation`.
- WHEN `ApplyMatch` grava stats em `game_player_stats` -> THE SYSTEM SHALL
  usar `m.ModeID` (não mais `""` hardcoded).
- Dado legado (partidas processadas antes desta correção) permanece com
  `mode_id=''` — sem migration de backfill (mesmo padrão de "bucket vazio"
  já aceito no resto do projeto pra dado anterior a uma dimensão nova).

## Contratos de Interface

### Domínio (analytics-service)
```go
// MatchResult ganha o campo que os "primos" já tinham (spec 010):
type MatchResult struct {
    MatchID string
    GameID  string
    ModeID  string // ADR-0024; "" = evento anterior a esta correção (dado legado)
    Results []PlayerResult
}

type MatchRepository interface {
    // ...
    // GameAnalytics devolve o resumo do jogo + as séries dos últimos `days`.
    // modeID == "" agrega TODOS os modos (comportamento atual, default);
    // modeID != "" filtra as séries mode-scoped (spec 012). Matches/
    // Participations/Wins/Losses/Draws/AvgScore/Placement/Mission NUNCA são
    // filtrados por modo (tabelas sem essa dimensão — item D, fora de
    // escopo desta spec).
    GameAnalytics(ctx context.Context, gameID string, days int, modeID string) (GameAnalytics, error)
}

// GameAnalytics ganha o campo que documenta o filtro aplicado:
type GameAnalytics struct {
    // ... campos existentes, inalterados
    // ModeID é o filtro efetivamente aplicado ("" = todos os modos). Eco do
    // parâmetro de entrada — permite o cliente confirmar o que recebeu sem
    // adivinhar pelo conteúdo (ex.: séries vazias podem ser "sem dado" OU
    // "filtro não bateu com nenhum modo", indistinguíveis sem o eco).
    ModeID string
}
```

### HTTP admin (aditivo)
```
GET /admin/games/{gameId}/analytics?days=30&modeId=mundo-vivo
```
`modeId` ausente ou vazio = comportamento atual (todos os modos). Resposta
ganha `"modeId"` no JSON (eco do filtro, omitido quando vazio).

## Critérios de Aceite

1. Chamada sem `modeId` -> resposta idêntica à atual, byte-a-byte nos campos
   existentes (regressão zero).
2. Jogo com 2 modos reportando a MESMA `stat_key` -> `?modeId=<modo-A>`
   devolve só a soma do modo A (hoje soma os dois).
3. `?modeId=<modo-sem-dado>` -> séries/stats vazias, sem erro 4xx/5xx.
4. `PlacementDaily`/`MissionDaily`/`ParticipationDaily`/`RoleParticipations`
   respeitam o filtro; `Matches`/`Participations`/`Placement`/`Mission`
   (cumulativos) continuam globais mesmo com filtro — documentado, não bug.
5. Validado com dado real: jogo com 2+ modos ativos (ex.: Achievements
   Test: `arena-ranqueada` + `mundo-vivo`), comparando resposta com e sem
   `modeId`.
6. Partida `queue_arena`/`win_loss_draw` real com stats -> `game_player_stats`
   grava o `mode_id` correto (não mais `''`); `?modeId=<modo-real>` isola
   corretamente as stats desse modo.

## Fora de escopo

- **UI (seletor de modo, reorganização visual)** — item B da ordem
  combinada; esta spec só entrega a capacidade no backend.
- **`mode_id` nos rollups cumulativos** (`game_rollup`,
  `game_placement_rollup`, `game_mission_rollup`) — item D, adiado.
- **Ingestão de session history pro Analytics** (session health) — item C,
  adiado.
