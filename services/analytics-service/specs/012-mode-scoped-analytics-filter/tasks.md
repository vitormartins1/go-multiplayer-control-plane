# Tasks — 012 filtro de modo no Analytics

- [x] 1. `internal/domain/analytics.go`:
  - [x] 1.1. `MatchRepository.GameAnalytics` ganha `modeID string`
  - [x] 1.2. `GameAnalytics` struct ganha `ModeID string`
  - [x] 1.3. `MatchResult` ganha `ModeID string` (achado adicional)
- [x] 2. `internal/repository/postgres_analytics.go`:
  - [x] 2.1. `GameAnalytics(ctx, gameID, days, modeID)`: seta `out.ModeID`
  - [x] 2.2. `dailySeries` ganha parâmetro `modeID`, WHERE ganha `AND ($4 =
        '' OR mode_id = $4)`; as 3 chamadas (placement/mission/participation
        daily) repassam
  - [x] 2.3. Query de `RoleParticipations` ganha `AND ($3 = '' OR mode_id =
        $3)`
  - [x] 2.4. Query de `Stats` ganha `AND ($3 = '' OR mode_id = $3)`
  - [x] 2.5. `ApplyMatch` (achado adicional): `insertPlayerStats(..., m.
        ModeID, ...)` em vez de `""` hardcoded
- [x] 3. `internal/transport/http_handler.go`:
  - [x] 3.1. Reader interface (`GameAnalytics`) ganha `modeID string`
  - [x] 3.2. Handler lê `r.URL.Query().Get("modeId")`, repassa
  - [x] 3.3. DTO de resposta ganha `ModeID string
        \`json:"modeId,omitempty"\``
- [x] 4. Atualizar `internal/service/pool_test.go` (`fakeRepo.GameAnalytics`)
      pra nova assinatura
- [x] 4.1. `internal/consumer/consumer.go` (achado adicional): `parse()`
      extrai `modeId` do envelope pra `domain.MatchResult.ModeID`
- [x] 5. Testes:
  - [x] 5.1. Repository (integration, testcontainers): 2 modos com a mesma
        `stat_key` -> filtro isola corretamente (`TestGameAnalytics_
        ModeID_GravaEIsolaStatsDoQueueArena`); séries diárias idem
        (`TestGameAnalytics_ModeIDFiltro_IsolaSeriesDiarias`); sem filtro ->
        soma como hoje (regressão zero, mesmos testes)
  - [x] 5.2. Transport: query param lido e ecoado no DTO
        (`TestGameAnalytics_ModeIdQuery_RepassaEEcoa`); ausente -> `ModeID`
        omitido (`TestGameAnalytics_SemModeIdQuery_CampoOmitido`)
  - [x] 5.3. `consumer_test.go` (achado adicional): `TestParse_LeModeID` +
        `TestParse_SemModeID_CaiEmVazio`
- [x] 6. `go build ./...` + `go test ./... -race -cover` no analytics-service
      (35 unit + 28 integration, todos passando)
- [x] 7. Validação ao vivo: Packet Arena (`default` FFA + `teams-2v2`),
      ambos com a MESMA `stat_key` (`orbs`) reportada via `ReportResult`
      real (grpcurl) — sem filtro soma tudo (`175`, inclui dado legado
      pré-fix); `?modeId=default` isola `12` (exatamente o esperado, 10+2);
      `?modeId=teams-2v2` isola `152` (100+50+1+1) — prova em produção que
      o bug de mistura entre modos foi corrigido e o filtro funciona
- [x] 8. admin-bff (spec-irmã 026): `Client`/`Service.GameAnalytics` ganham
      `modeID`; handler lê `modeId` da query; `GameAnalyticsDetail` ganha
      `ModeID`; validar ao vivo contra o analytics-service real
