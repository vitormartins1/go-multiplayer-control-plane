# Tasks — 009 quebra por modo nas participações

## analytics-service

- [x] 1. Migration `007_participation_mode_id.sql`: adiciona `mode_id`
      (default `''`) e troca a PK de `game_participation_rollup` para
      `(game_id, mode_id, role, event, outcome)`.
- [x] 2. `consumer.go`: `parseParticipation` extrai `modeId` do envelope;
      `domain.ParticipationEvent` ganha o campo.
- [x] 3. `domain/analytics.go`: `ParticipationStat` ganha `ModeID`.
- [x] 4. `postgres_analytics.go`: `ApplyParticipation` grava `mode_id` no
      INSERT/ON CONFLICT; leitura de `GameAnalytics` seleciona e escaneia
      `mode_id`.
- [x] 5. `http_handler.go`: `participationStatDTO` ganha `modeId` no JSON.
- [x] 6. Testes: consumer (com/sem modeId), repository (dois modos não
      somam entre si; round-trip da migration), http handler (DTO expõe o
      campo). Rodar `go test ./services/analytics-service/... -race`.

## admin-bff

- [x] 7. `ParticipationStat` (domain) ganha `ModeID` — passthrough.
      `go test ./services/admin-bff/... -race`.

## admin-ui

- [x] 8. `api.ts`: `ParticipationStat.modeId: string`.
- [x] 9. `GameAnalytics.tsx`: busca os modos do jogo (via prop ou
      `api.listMultiplayerModes`), deriva a lista de modos host_world_dropin
      presentes no dado, adiciona o seletor (default "Todos os modos"),
      filtra gráfico + tabela pelo modo escolhido.
- [x] 10. Build limpo (`npm run build`), rebuild dos 3 containers, seed de
       um evento de participação num SEGUNDO modo host_world_dropin (via
       grpcurl, mesmo caminho já usado nesta sessão) pra validar o filtro
       com 2 modos reais, screenshot de verificação.
