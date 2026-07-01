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

## Specs preparadas — ✅ TODAS CONCLUÍDAS (2026-06-30)

> Padrão de referência já no repo: `achievements/004-game-scope`, `achievements/003-admin-read`, `leaderboard/002-game-scope`, `admin-bff/005-leaderboard-panel`.

1. ~~**`services/player-data-service/specs/002-game-scope`**~~ — ✅ **feito**: game_id na PK + admin read (`GET /admin/games/{g}/players/{p}/data`) + guard internal key.
2. ~~**`services/cloud-save-service/specs/002-game-scope`**~~ — ✅ **feito**: key S3 `{gameID}/{playerID}` + admin read (`GET /admin/games/{g}/players/{p}/save`, metadados + blob base64) + guard.
3. ~~**`services/admin-bff/specs/006-player-tabs`**~~ — ✅ **feito**: abas **Player data** e **Cloud saves** do jogador no painel (bff + ui), consumindo os admin reads de 1 e 2.

Com isso, **todo o detalhe do jogador está aceso no painel** (Achievements + Player data + Cloud saves).

## Painel jogo-cêntrico (melhoria de UX/arquitetura — 2026-06-30)

Sequência iniciada a pedido do usuário (biblioteca de capas + repensar a seleção de jogo). Faseada:

- ✅ **Fase A** (`admin-ui/005-game-library`): Games virou **biblioteca de capas 9:16** (procedural); **jogo ativo global** no header (`GameProvider`/`useGames`, persistido); abas do jogador consomem o contexto (sem `<Select>` repetido).
- ✅ **Fase B** (`game-registry/002-game-presence`, **ADR-0019**): **player base por jogo** via **presença** (game-registry `game_players`), marcada no **login-into-game** (identity aceita `gameId`, marca best-effort); admin-bff `GET /api/games/{g}/players`; aba **Players** virou jogo-cêntrica (base do jogo ativo).
- ✅ **Navegação player-cêntrica** (`game-registry/004-games-of-player`): **review do modelo** — identidade global + presença (ADR-0017/0019) está certa; o ruído era o "jogo ativo global" no header (removido). Agora **dois eixos**: **Games** (abre um jogo → aba Players = base do jogo/presença) e **Players** (base global do identity, com avatares) → abrir um player → **seletor escopado aos jogos DELE** (`GET /api/players/{id}/games`, consulta reversa). Sem `GameProvider`/`GameSwitcher`.
- ✅ **Avatares** (`admin-ui/006-player-avatars`): DiceBear `adventurer` por jogador (seed = playerId).
- ✅ **Fase C** (`game-registry/003-generated-cover`): **capa determinística por seed** — game-registry guarda metadados (`image`, `imageSource='generated'`, `generatedImageSeed=slug`, `generatedImageStyle='shapes'`); o admin-ui gera a imagem no cliente via **DiceBear** (`GameCover`). Abordagem "cliente gera" (não backend). Espaço p/ upload futuro (`image`/`imageSource`) e troca de estilo deixados abertos. Registros antigos via COALESCE.

## Backlog (sem spec ainda)

Service JWT (evolução do 0018), fan-out resiliente (outbox/retry), leaderboard seasons/paginação, **analytics-service** (Fase 9 do ROADMAP — único serviço não construído; acende a aba Analytics), infra/deploy (Terraform → AWS, a tese de custo), **cleanup admin-bff** (helper `internal/upstream` compartilhado: `do()`/`classify()`/`errorCode()`/default-client hoje duplicados nos 4 adapters + presence client do identity — do code-review). Implementar uma feature por branch `feat/<serviço>/<NNN-slug>`, merge `--no-ff`, conforme `docs/GIT-WORKFLOW.md`.

**Painel — CRUD:** ✅ **editar/remover conquistas** (`achievements/006-catalog-crud`). ⏳ **editar/remover jogo** pendente — *rename* é simples (cabe na aba Configurações do GameDetail, hoje ComingSoon); *delete* precisa de decisão (dado órfão por `game_id` em achievements/player-data/cloud-save/leaderboard/presence — cascata cross-service, provável ADR).

## CI (GitHub Actions) — ✅ criado (`.github/workflows/ci.yml`)

Jobs em push na `main` e em PR: **go** (build/vet/`test -race -cover`), **integration** (`test -tags=integration`, testcontainers), **lint** (`golangci-lint` v2.12.2 pinado + `.golangci.yml`) e **frontend** (`npm ci` + build). Um job Go cobre tudo (módulo único, ADR-0001). Reproduzido verde localmente.

**Pendente do usuário:** o repo ainda é **local** — o CI só dispara depois de criar `github.com/vitormartins1/go-game-backend` e fazer o `git push`. O module path já foi renomeado de `c130n/game-backend` para `vitormartins1/go-game-backend`.

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
