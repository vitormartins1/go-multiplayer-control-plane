# Spec: 016-analytics-outcome

**Serviço:** admin-ui (+ ajuste no admin-bff)
**Status:** Draft
**Data:** 2026-07-06
**Parte de:** B2a do roadmap `queue_arena` (ADR-0023). O analytics-service já agrega `losses`/`draws` (fatia analytics/003).

## Contexto

O analytics-service emite `losses`/`draws` por jogo (empate deixou de contar como derrota — analytics/003). Mostrar isso no console. **Correção sobre o roadmap:** o admin-bff **não** era passthrough cru aqui — o `domain.AnalyticsGameStat` é uma struct tipada que **descartava** `losses`/`draws` no decode/re-encode. Então a fatia também toca o BFF (adicionar os dois campos), além do admin-ui.

## Comportamento

- THE SYSTEM SHALL, na aba **Analytics** (overview cross-game), incluir colunas **Derrotas** e **Empates** na tabela "Jogos por partidas", ao lado de Vitórias.
- THE SYSTEM SHALL, na aba **Analytics por jogo** (`GameAnalytics`), incluir StatCards de **Derrotas** e **Empates** ao lado de Vitórias.

## Contratos de Interface

- **admin-bff:** `domain.AnalyticsGameStat` +`Losses int64` +`Draws int64` (json `losses`/`draws`). O adapter `analytics/client.go` decodifica direto nesses campos — nenhuma outra mudança.
- **admin-ui:** `AnalyticsGameStat` +`losses: number` +`draws: number`; `Analytics.tsx` (+2 colunas) e `GameAnalytics.tsx` (+2 StatCards).

## Casos de Teste / Verificação

- [ ] (admin-bff) `analytics/client_test`: `losses`/`draws` do JSON chegam no domínio
- [ ] `npm run build` (typecheck) limpo; overview e por-jogo mostram derrotas/empates

## Fora do escopo

- Seletor de board no leaderboard (B2b, próxima fatia).
- Stats por partida no analytics (follow-up do ADR-0021).
