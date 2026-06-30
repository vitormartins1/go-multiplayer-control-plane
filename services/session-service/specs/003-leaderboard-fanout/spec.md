# Spec: 003-leaderboard-fanout

**Serviço:** session-service
**Status:** Approved
**Data:** 2026-06-30

## Contexto

Estende o fan-out do resultado (002) para **também** alimentar o leaderboard, além do achievements. Introduz um **CompositePublisher** que distribui o resultado para N destinos (fan-out para N sinks), mantendo o `ReportResult` com um único `Publish`. Cada destino é best-effort e autenticado pela internal key (ADR-0018). O leaderboard ganhou o submit server-side na sua 002-game-scope.

## Comportamento

- WHEN `ReportResult` dispara o fan-out -> o resultado vai para **todos** os destinos configurados (achievements + leaderboard).
- THE SYSTEM SHALL enviar ao leaderboard o **score** de cada jogador (`POST /admin/games/{gameId}/players/{playerID}/scores {score}`) no jogo da sessão, com a internal key.
- THE SYSTEM SHALL manter o achievements (`match_played`/`match_won`, da 002) inalterado.
- THE SYSTEM SHALL ser **best-effort por destino**: a falha de um sink (ou de um jogador) não impede os outros nem falha o `ReportResult` (loga).
- A escolha dos destinos é por configuração: cada um liga via sua env de URL (`ACHIEVEMENTS_URL`, `LEADERBOARD_URL`); sem nenhuma, cai no fallback de log.

## Contratos de Interface

- Novo adapter `internal/publisher/leaderboard_publisher.go` (HTTP) satisfaz `domain.ResultPublisher`; envia `r.Score` por jogador.
- Novo `internal/publisher/composite.go` — `Composite` satisfaz `domain.ResultPublisher` e delega para uma lista de publishers (best-effort).
- `ResultPublisher.Publish(ctx, gameID, results)` inalterado (o Composite é transparente).

## Casos de Teste Obrigatórios

- [ ] `LeaderboardPublisher` envia `score` por jogador para `/admin/games/{g}/players/{p}/scores`
- [ ] `Composite` chama **todos** os publishers (achievements + leaderboard)
- [ ] um publisher falhando não impede o outro (best-effort) nem retorna erro
- [ ] o fan-out usa o `game_id` da sessão e a internal key

## Fora do escopo

- **Consumo no admin-bff/admin-ui** (aba Leaderboards) — feature seguinte
- Mapeamento rico de score (agregação/normalização) — usa `r.Score` direto
- player-data/cloud-save no fan-out — futuro (entram na lista do Composite)
