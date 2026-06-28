# Plan: 001-achievement-core
**Spec:** specs/001-achievement-core/spec.md
**Data:** 2026-06-28

## Abordagem técnica

API REST com `net/http` (ServeMux 1.22, method patterns), hexagonal (ADR-0002): `domain` define `Achievement`/`Unlock`/`Event` e as Ports; `service` tem a regra de desbloqueio; `repository` persiste em Postgres; `transport` traduz HTTP e aplica o middleware de auth. A validação do JWT do identity vive num adapter de saída (`internal/auth`), que implementa a Port `TokenVerifier`.

Fluxo do desbloqueio: `POST /events` → middleware valida o Bearer JWT (via JWKS) e injeta o `playerID` no contexto → `RecordEvent(ctx, playerID, event)` busca as conquistas cujo `event_type` casa o evento → para cada uma, `Unlock` idempotente → retorna as recém-desbloqueadas. O core **não conhece HTTP nem o token**: recebe `playerID` já resolvido, então trocar a ingestão por um consumer de fila no futuro não toca o domínio (ver decisão de ingestão na spec).

## Compute e banco (ADR-0005, ADR-0009, ADR-0015)

- **Banco: PostgreSQL próprio do serviço** (database-per-service, ADR-0015; par AWS na tese = DynamoDB). Local: container `achievements-postgres` separado, isolando o schema do identity. Sem `game_id` no MVP (evolução multi-jogo conhecida, ADR-0015).
- **Local:** container no docker-compose; testes de integração com `testcontainers-go` (Postgres real e descartável).
- **Cloud/deploy:** fora do escopo (estratégia local-first; Terraform/deploy no fim do projeto).

## Arquivos a criar ou modificar

- `services/achievements-service/internal/domain/achievement.go` — `Achievement`, `Unlock`, `PlayerAchievement`, `Event` + Ports (`AchievementService`, `AchievementRepository`, `TokenVerifier`)
- `services/achievements-service/internal/domain/errors.go` — `ErrUnauthorized`, `ErrInvalidEvent`
- `services/achievements-service/internal/service/achievement_service.go` — `ListCatalog`, `ListForPlayer`, `RecordEvent`
- `services/achievements-service/internal/service/achievement_service_test.go` — unit (mocks de repo)
- `services/achievements-service/internal/repository/postgres_achievement.go` — pgx (catálogo, unlocks, unlock idempotente)
- `services/achievements-service/internal/repository/postgres_achievement_test.go` — integration (testcontainers)
- `services/achievements-service/internal/auth/jwks_verifier.go` — `TokenVerifier` via JWKS do identity (cache por `kid`)
- `services/achievements-service/internal/auth/jwks_verifier_test.go` — token válido/expirado/assinatura errada/kid desconhecido
- `services/achievements-service/internal/transport/http_handler.go` — handlers, middleware de auth, erro→HTTP
- `services/achievements-service/internal/transport/http_handler_test.go` — httptest (com token de teste)
- `services/achievements-service/migrations/001_create_achievements.sql` — schema + **seed do catálogo**
- `services/achievements-service/cmd/server/main.go` — wire-up, tracing, otelpgx, graceful shutdown
- `services/achievements-service/Dockerfile`
- `services/achievements-service/api/achievements.http` — collection de teste (workflow .http)
- `shared/config/config.go` — adicionar `IdentityJWKSURL` (env `IDENTITY_JWKS_URL`)
- `infra/docker/docker-compose.yml` — `achievements-service` + `achievements-postgres`
- `infra/docker/prometheus.yml` — job de scrape do achievements

## Dependências e libs

- `github.com/jackc/pgx/v5` — Postgres (stack do projeto)
- `github.com/golang-jwt/jwt/v5` — **validação** do JWT EdDSA (mesma lib que o identity usa para emitir; já no go.mod)
- JWKS parseado **à mão** (decodifica `x` base64url → `ed25519.PublicKey`); sem lib de JWK nova — enxuto e didático
- `github.com/exaring/otelpgx`, `otelhttp`, `shared/telemetry`, `shared/middleware`, `shared/logger` — observabilidade herdada (já existem)
- Testes: `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go`
- **Nenhuma dependência nova** além das já presentes no módulo

## Decisões técnicas e trade-offs

- **Onde mora o verificador de JWKS — `internal/auth` agora (não `shared/auth`):** o achievements é o **primeiro** consumidor do token do identity; vou implementar local e **promover a `shared/auth` quando o 2º consumidor (player-data) chegar**, quando a forma comum estiver comprovada. Evita cravar abstração compartilhada com base em um único caso e não toca a fundação `shared/` cedo demais.
- **Cache de JWKS:** `map[kid]ed25519.PublicKey` com TTL (ex. 5 min) e **refetch sob `kid` desconhecido** (cobre rotação de chave sem reiniciar). Fetch **lazy** (no primeiro uso), para o serviço subir mesmo que o identity ainda não esteja pronto.
- **Idempotência atômica no banco:** `INSERT ... ON CONFLICT (player_id, achievement_id) DO NOTHING RETURNING ...` — se retornou linha, foi um desbloqueio novo; se não, já existia. Sem ler-antes-de-escrever, sem corrida (mesma lição do GO-LEARNINGS #01, agora resolvida no store).
- **Critério = `event_type`:** cada conquista tem um tipo de evento que a dispara; um evento casa 0+ conquistas. Modelo simples e extensível para contadores na feature 002.
- **Catálogo por seed:** `INSERT` das conquistas iniciais na própria migration; a API só lê.
- **`playerID` do `sub` do JWT:** resolvido no transport e passado ao service; o domínio nunca vê o token (mantém `domain`/`service` sem dependência de HTTP/JWT).
- **Observabilidade de nascença:** herda `shared/middleware` (RED + exemplars), `shared/telemetry` (tracing W3C) e otelpgx — o serviço sobe instrumentado, sem retrabalho.

## Ordem de implementação

1. `domain` (tipos, Ports, errors) — base sem dependências
2. `internal/auth` (JWKS verifier) + testes — pura, destrava o middleware
3. `service` (`ListCatalog`, `ListForPlayer`, `RecordEvent`) + unit tests (mock repo)
4. `repository` Postgres + migration/seed + integration tests (testcontainers)
5. `transport` (handlers, middleware de auth, erro→HTTP) + httptest
6. wire-up `cmd/server/main.go` + Dockerfile + `achievements-service`/`achievements-postgres` no compose + job no Prometheus
7. collection `api/achievements.http`
8. `go test ./... -race` e `go build ./...` verdes; validar o loop .http → Grafana/Tempo → banco

## Riscos e pontos de atenção

- **Boot ordering (identity ↔ achievements):** o achievements precisa do JWKS do identity. Mitigado pelo fetch **lazy** + cache (não falha o boot; busca no primeiro request e em `kid` desconhecido). `depends_on` ajuda mas não garante readiness.
- **Trust boundary:** no MVP o jogador autenticado posta o próprio evento (mock); o emissor confiável (session/game server) vem depois. Documentado na spec.
- **Expiração/clock skew:** validar `exp` do JWT; tolerar skew pequeno se necessário.
- **testcontainers exige Docker** na máquina de dev/CI.

## Decisões confirmadas (antes resididas em revisão)

1. **Verificador de JWKS em `internal/auth`** agora, com promoção a `shared/auth` no 2º consumidor.
2. **Banco próprio do achievements** — container `achievements-postgres` separado (database-per-service, ADR-0015).
