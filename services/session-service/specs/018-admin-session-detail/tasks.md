# Tasks — 018-admin-session-detail

- [x] session-service: `domain.Session.Result *MatchOutcome`; `ReportResult` grava pós-publish (sob lock, antes do `closeSession`) + teste (`Result` gravado; publish falho → nil; ttl/admin → nil)
- [x] session-service: DTO admin `archetype`/`profileHash`/`modeParams`/`teams`/`result` (tagged union espelhada, `omitempty`) + testes de serialização (com/sem snapshot, com/sem result, FFA sem teams)
- [x] admin-bff: `domain.GameSession` + client do session repassam os campos + teste de decode
- [x] admin-ui: tipos em `lib/api.ts`; expand com arquétipo/hash, chips de params, formação por time, tabela de resultado por kind
- [x] Verificação: `go test ./services/{session-service,admin-bff}/... -race` · `npm run build` no admin-ui · e2e manual com partida `teams-2v2` real
