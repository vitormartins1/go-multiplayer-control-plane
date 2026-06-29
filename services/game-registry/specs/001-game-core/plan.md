# Plan: 001-game-core
**Spec:** specs/001-game-core/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Serviço REST (`net/http`), hexagonal (ADR-0002), no mesmo esqueleto do identity (sem token issuer): `domain` (tipos + Ports + erros), `service` (validação + regras), `repository` (pgx/Postgres próprio), `transport` (handlers). Banco próprio `game_registry` (database-per-service, ADR-0015). Entra na observabilidade como serviço novo (RED + scrape + exporter do banco) e vira um nó na topologia do painel.

`slug` é a chave humana estável, com unicidade **case-insensitive** garantida por índice em `lower(slug)` (mesmo padrão do `handle` do identity); `id` UUID é a chave canônica usada pelos outros serviços.

## Arquivos a criar ou modificar

**Serviço (novo)**
- `internal/domain/game.go` — `Game{ID,Slug,Name,CreatedAt}`; Ports `GameService` (`Register`/`List`/`Get`) e `GameRepository` (`Create`/`List`/`FindByID`)
- `internal/domain/errors.go` — `ErrInvalidSlug`, `ErrInvalidName`, `ErrSlugTaken`, `ErrGameNotFound`
- `internal/service/game_service.go` (+ test) — `Register` (valida slug/name, gera `uuid`/`createdAt`), `List`, `Get`; unit tests com mock repo (slug/name inválidos; slug tomado; get inexistente)
- `internal/repository/postgres_game.go` (+ integration test, testcontainers) — `Create` (map `23505` -> `ErrSlugTaken`), `List` (order by `name`), `FindByID` (`ErrGameNotFound`)
- `internal/transport/http_handler.go` (+ test, httptest) — `POST /games`, `GET /games`, `GET /games/{id}`; DTO; mapeia erros -> 400/404/409/201
- `cmd/server/main.go` — wire-up: `pgxpool` + `otelpgx` (spans de query), service, transport, `middleware.NewMetrics` + `RouteSpanName`, `otelhttp`, `/healthz`, `/metrics`, graceful shutdown
- `migrations/001_create_games.sql` — tabela `games` + índice único em `lower(slug)`
- `services/game-registry/Dockerfile` — multi-stage (igual aos demais Go)
- `services/game-registry/api/game-registry.http` — coleção (registrar/listar/consultar)
- `services/game-registry/README.md`

**Infra / observabilidade (ritual do serviço novo)**
- `infra/docker/docker-compose.yml` —
  - `game-registry` (HTTP host **8090**; `DATABASE_URL` -> `game-registry-postgres`; depends_on postgres healthy + tempo)
  - `game-registry-postgres` (postgres:16, db `game_registry`, host **5435**, monta a migration como init script, healthcheck)
  - `game-registry-postgres-exporter` (postgres-exporter, depends_on o postgres)
- `infra/docker/prometheus.yml` — jobs `game-registry` (serviço) e `game-registry-postgres` (exporter)

**Painel (mostra o serviço novo)**
- `services/admin-bff/internal/domain/topology.go` — adicionar o nó `game-registry` em `KnownServices` (aparece na topologia do `admin-ui`)

## Compute e banco (ver ADR-0005)
- **Banco:** Postgres próprio (`game_registry`), container dedicado — database-per-service (ADR-0015). CRUD relacional simples; Postgres é o default natural.
- **Compute:** diferido (ADR-0005); localmente é mais um container.

## Dependências e libs
- `github.com/jackc/pgx/v5` (pool + erros), `github.com/google/uuid`, `github.com/exaring/otelpgx` — todos já no módulo. Sem dep nova.

## Decisões técnicas e trade-offs
- **Unicidade de `slug` por `lower(slug)`** (índice único), não por coluna `citext`: mesmo padrão do identity (`handle`), sem extensão extra. O service valida o formato; o banco garante a unicidade (`23505` -> `ErrSlugTaken`).
- **`id` UUID canônico, `slug` humano**: os outros serviços referenciam `game_id` (UUID, estável); o `slug` é pra humanos/URLs. Evita acoplar `game_id` a um texto editável.
- **Sem auth (interno)**: control plane, como os `/admin` dos outros. Rotas em `/games` (o serviço inteiro é interno; não há superfície pública pra distinguir).
- **`List` por `name` asc**: catálogo pequeno, ordenação amigável (≠ identity, que é `created_at` desc por ser timeline de cadastro).
- **Nó na topologia agora**: uma linha em `KnownServices` do admin-bff já faz o serviço aparecer no painel; a aresta `admin-bff → game-registry` entra quando o `admin-bff/004-games` consumir o registry.

## Ordem de implementação
1. `domain` (tipos, Ports, erros)
2. `migrations/001_create_games.sql`
3. `repository` (pgx) + integration test (testcontainers): create/slug-taken, list por nome, find/not-found
4. `service` (validação + Register/List/Get) + unit tests
5. `transport` (rotas + DTO + mapeamento de erro) + handler tests
6. `cmd/server/main.go` (wire-up) + Dockerfile + README + api/.http
7. compose (serviço + postgres + exporter) + prometheus jobs + nó na topologia
8. `go test ./... -race` + `go test -tags=integration ./.../repository/`; `go build ./...`; subir no Docker e validar (registrar/listar/consultar; banco e métricas no Prometheus)

## Riscos e pontos de atenção
- **Portas novas:** HTTP **8090**, Postgres host **5435** — confirmadas livres (próximas na sequência).
- **`id` do nó na topologia == job do Prometheus** (`game-registry`) — senão o status `up` não casa.
- **Validação de `slug`** com regex robusta (minúsculas, `[a-z0-9-]`, sem hífen nas pontas, 2–40) — cobrir os casos de borda no teste.
- **Init script idempotente:** a migration roda na criação do volume; em rebuilds com volume existente não re-roda (padrão do projeto — `docker compose down -v` para resetar).
- **Observabilidade:** não esquecer o exporter do banco + os dois jobs (serviço e `-postgres`), conforme `docs/OBSERVABILITY.md`.
