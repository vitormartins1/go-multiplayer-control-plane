# Tasks: 023-fleet-status
**Plan:** specs/023-fleet-status/plan.md
**Data:** 2026-07-11

## Fase 1 — session-service: domain + adapter ✅
- [x] `internal/domain/session.go`: `FleetStatus` struct + `FleetInspector` interface
- [x] `internal/allocator/agones.go`: `fleetGVR`; método `FleetStatus(ctx, fleetName)` (Get + NotFound tratado + parse dos campos numéricos)
- [x] Unit test do adapter com fake dynamic client: Fleet existente, Fleet inexistente

## Fase 2 — session-service: transport + wiring ✅
- [x] `internal/transport/http_handler.go`: `AdminHandler` ganha `fleets domain.FleetInspector` + `clusterLabel string`; rota `GET /admin/fleet-status/{fleetName}`; handler cobre os 3 casos da spec (sem Agones / Agones sem a fleet / Agones com a fleet)
- [x] Handler test cobrindo os 3 casos (EARS 2/3/4)
- [x] `cmd/server/main.go`: lê `AGONES_CLUSTER_LABEL`; injeta o adapter Agones como `FleetInspector` só quando o allocator ativo é Agones
- [x] `infra/docker/docker-compose.agones.yml` e `docker-compose.agones-pi5.yml`: `AGONES_CLUSTER_LABEL: local` / `pi5`

## Fase 3 — admin-bff ✅
- [x] `internal/domain/sessions.go`: `FleetStatus` (DTO local) + método adicionado à Port `SessionsReader` (reaproveitada, não Port nova — mesmo client/serviço)
- [x] `internal/session/client.go`: método `FleetStatus(ctx, fleetName)`, com teste (server httptest)
- [x] `internal/service/admin_service.go`: resolução `gameID → fleet` (via `ListGames`) + chamada; caso "sem fleet cadastrada" sem round-trip; teste
- [x] `internal/transport/http_handler.go`: rota `GET /api/games/{gameId}/fleet-status` (prefixo `/api` — convenção real do admin-bff, spec tinha o path sem o prefixo); teste de handler (200, session-service fora)

## Fase 4 — admin-ui ✅
- [x] `src/lib/fleetLocation.ts`: tipo + color/label helpers (offline/local/pi5/unknown)
- [x] `src/components/FleetLocationBadge.tsx`
- [x] `src/lib/api.ts`: tipo `FleetStatus` + `api.fleetStatus(gameId, signal)`
- [x] `src/pages/Fleet.tsx`: `FleetRealStatus` só é montado dentro do `Collapse` da linha expandida (dispara no expand, não no mount da lista) — nova seção "Onde está rodando" (badge + réplicas reais) ao lado da seção mockada existente
- [x] `tsc --noEmit` + `npm run build` limpos

## Fase 5 — Verificação
- [x] `go test ./services/{session-service,admin-bff}/... -race` limpo (352 testes)
- [x] Smoke via API (curl, não browser — sem ferramenta de automação de navegador nesta sessão): override `agones.yml` (k3d) → `GET /admin/fleet-status/capoeira-souls` no session-service e `GET /api/games/{id}/fleet-status` no admin-bff devolvem `cluster=local, fleetExists=true, ready=2/desired=2`
- [x] Smoke via API: override `agones-pi5.yml` → mesmo endpoint devolve `cluster=pi5, fleetExists=true` (Fleet real do Pi, aplicada na Fatia 3 da spec 005 do repo Unity)
- [x] Smoke via API: jogo sem fleet cadastrada → `cluster="", fleetExists=false` sem chamar o session-service
- [x] Smoke via API: session-service parado (`docker stop`) → `GET /api/games/{id}/fleet-status` devolve 502 (o admin-ui mapeia isso pro badge "desconhecido" — não verificado visualmente no browser)
- [ ] **Pendente do usuário:** confirmação visual no browser (`localhost:8089` → aba Fleet) — a lógica de UI foi type-checked/buildada mas não clicada de verdade
