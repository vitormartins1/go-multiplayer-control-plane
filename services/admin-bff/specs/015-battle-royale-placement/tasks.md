# Tasks: 015-battle-royale-placement

**Serviço:** admin-bff · **Spec:** [spec.md](./spec.md)

- [x] **AB1** — `internal/domain/analytics.go`: `PlacementDistribution{Winners,TopN,Rest,Entries int64; AvgSurvivedMs float64}` + `Placement *PlacementDistribution` (`placement,omitempty`) em `GameAnalyticsDetail`.
- [x] **AB2** — `internal/analytics/client_test.go`: parse de `placement` no detalhe; ausência → nil.
- [x] **AB3** — `go build ./...`; `go test ./services/admin-bff/... -race -cover`; `golangci-lint run ./services/admin-bff/...`; `gofmt` nos arquivos tocados.
