# Plan: 003-players
**Spec:** specs/003-players/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Mesmo padrão do adapter de achievements (admin-bff/001): novo Port de saída `PlayerDirectory`, adapter HTTP que chama o `GET /admin/players` do identity, método de service `Players(ctx, limit, offset)` e rota `GET /api/players` no transport. Config: `IDENTITY_URL` (lido no `main`, como `PROMETHEUS_URL`/`ACHIEVEMENTS_URL`). Sem banco.

## Arquivos a criar ou modificar

- `internal/domain/admin.go` — tipos `Player{ID,Handle,CreatedAt}`, `PlayerPage{Total,Players}`; Port `PlayerDirectory { ListPlayers(ctx, limit, offset) (PlayerPage, error) }`
- `internal/domain/errors.go` — `ErrIdentityUnavailable` (transport -> 502)
- `internal/identity/client.go` (+ test) — adapter: `GET {IDENTITY_URL}/admin/players?limit=&offset=`, mapeia `{total, players[]}` -> `PlayerPage`; conexão/HTTP fora -> `ErrIdentityUnavailable`; `httptest`
- `internal/service/admin_service.go` — método `Players(ctx, limit, offset)` (delega ao directory)
- `internal/service/admin_service_test.go` — delega + repassa limit/offset (fake directory)
- `internal/transport/http_handler.go` — `AdminService` += `Players`; rota `GET /api/players` (parse limit/offset; `ErrIdentityUnavailable`->502)
- `internal/transport/http_handler_test.go` — 200 (página) + 502 (identity fora)
- `cmd/server/main.go` — lê `IDENTITY_URL`; cria o `identity` client (otelhttp, como os outros); injeta no `service.New`
- `internal/service` `New(...)` ganha o `PlayerDirectory`
- `api/README.md` + `README.md` — documentar `/api/players`
- `infra/docker/docker-compose.yml` — `admin-bff`: `IDENTITY_URL: http://identity-service:8080` + `depends_on: identity-service`

## Decisões técnicas e trade-offs
- **Repassar paginação ao identity** (sem reclampar no BFF): o identity é a autoridade da política; o BFF é proxy fino. Evita regra duplicada.
- **Adapter próprio** (não reusar o de achievements): fontes diferentes, contratos diferentes; mantém um adapter por dependência.
- **`IDENTITY_URL` no `main`** (não em shared/config): consistente com as outras envs do BFF.
- **502 para identity fora**: mesmo padrão de "fonte de dado upstream indisponível" do achievements.

## Ordem de implementação
1. `domain` (tipos + Port + erro)
2. `identity` client + test (`httptest`: mapeia página; fora -> erro)
3. `service` (Players) + test
4. `transport` (rota + 502) + test
5. wire-up `main` + `service.New` + compose + READMEs
6. `go test ./... -race` + `go build ./...`; validar contra o identity real

## Riscos e pontos de atenção
- **Shape do JSON do identity:** `{ "total": int, "players": [ {id, handle, createdAt} ] }` — `createdAt` é RFC3339 string; mapear para `time.Time`.
- **Assinatura do `service.New`** muda (ganha o directory) — ajustar `main` e os testes do service/transport existentes.
- **otelhttp** no client do identity (trace cross-service).
