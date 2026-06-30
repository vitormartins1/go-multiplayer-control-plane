# NEXT-STEPS — handoff (2026-06-30)

Ponto de retomada após limpar o contexto. Resume **o que está pronto**, **as specs preparadas** (prontas para implementar) e **o backlog**.

## Estado atual (main verde, stack local de pé)

O **gameplay loop** está fechado e observável no painel:

`FindMatch(game_id) → match por jogo → CreateSession(game_id) → ReportResult → fan-out (achievements + leaderboard) → painel`

- **Multi-tenant por jogo** (ADR-0017) ponta a ponta: game-registry + achievements + leaderboard escopados por `game_id`.
- **Auth serviço-a-serviço** (ADR-0018): internal key (`X-Internal-Key`) nos `/admin`; `shared/auth` (`RequireInternalKey` + `InternalKeyTransport`). Chave dev no compose: `dev-internal-key`.
- **Fan-out** do resultado: `session` → `CompositePublisher` → achievements (match_played/match_won) + leaderboard (score). Best-effort.
- **Painel** (admin-bff + admin-ui, Mantine): Visão geral (mapa+saúde), Games (detalhe: Achievements, Leaderboards), Players (detalhe: Achievements; Player data/Cloud saves ainda "em breve"), Analytics ("em breve"). URL: http://localhost:8089.

ADRs relevantes recentes: 0016 (painel BFF), 0017 (multi-tenant game-registry), 0018 (service auth).

## Specs preparadas (prontas para implementar, nesta ordem)

> Padrão de referência já no repo: `achievements/004-game-scope`, `achievements/003-admin-read`, `leaderboard/002-game-scope`, `admin-bff/005-leaderboard-panel`.

1. ~~**`services/player-data-service/specs/002-game-scope`**~~ — ✅ **feito** (2026-06-30): game_id na PK + admin read (`GET /admin/games/{g}/players/{p}/data`) + guard internal key.
2. ~~**`services/cloud-save-service/specs/002-game-scope`**~~ — ✅ **feito** (2026-06-30): key S3 `{gameID}/{playerID}` + admin read (`GET /admin/games/{g}/players/{p}/save`, metadados + blob base64) + guard.
3. **`services/admin-bff/specs/006-player-tabs`** — acende as abas **Player data** e **Cloud saves** do jogador no painel (bff + ui). **Depende de 1 e 2 (ambas feitas) → próximo.**

Cada uma tem spec/plan/tasks. Implementar uma feature por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff`, conforme `docs/GIT-WORKFLOW.md`.

## CI (GitHub Actions) — ainda não existe `.github/workflows/`

A `main` está verde mas sem guarda automática. Plano (criar quando quiser):

- **Workflow `ci.yml`** em PR e push para `main`:
  - job **go**: setup Go 1.22, cache de módulos, `go build ./...`, `go vet ./...`, `go test ./... -race -cover` (testes de integração ficam atrás da tag `integration` — rodar só onde houver Docker disponível; ou um job separado com service containers).
  - job **lint**: `golangci-lint run ./...`.
  - job **front**: setup Node, `npm ci` + `npm run build` em `services/admin-ui`.
- Opcional: matriz/seletivo por serviço só faz sentido com multi-módulo; como é **módulo único** (ADR-0001), um job Go cobre tudo.
- Testes de integração usam **testcontainers** (Docker-in-Actions: usar runner com Docker; ou marcar como opcional).
- Não é fluxo SDD (infra). Pode virar um `docs/adr` se a escolha de pipeline for relevante.

## Backlog (ADR-level, quando sair do MVP)

- **Service JWT (evolução do ADR-0018)**: trocar a internal key compartilhada por tokens de serviço (identidade por serviço, expiração, revogação). Hoje é uma chave única (fail-closed). Abrir ADR quando houver múltiplos ambientes/consumidores.
- **Fan-out resiliente**: hoje é best-effort síncrono (loga falhas). Evoluir para **outbox/retry** (ou fila) para garantir entrega do resultado a achievements/leaderboard. Abrir ADR (provável SQS/outbox table).
- **Leaderboard**: seasons/janelas por período; paginação.
- **Painel**: editar/remover jogos e conquistas; Analytics real.

## Gotchas operacionais (já documentados, mas para lembrar)

- **Migrations Postgres** só rodam em **volume novo**: ao mudar/adicionar migration, `docker compose ... down -v` do `<svc>-postgres` (e remontar a migration no volume do compose).
- **Prometheus** não raspa job novo sem `docker compose restart prometheus`; **Grafana** provisioning só no boot (restart do container).
- **buf**: `PATH="$(go env GOPATH)/bin:$PATH" buf generate` dentro do serviço.
- **Processo órfão** segurando porta gRPC (50051/50052) de algum `go run` antigo: matar antes de `up`.
- **/metrics e /healthz** ficam **fora** do guard da internal key (Prometheus precisa raspar).
- Internal key dev: `dev-internal-key` (header `X-Internal-Key`).
