# Tasks: 023-fleet-status
**Plan:** specs/023-fleet-status/plan.md
**Data:** 2026-07-11

## Fase 1 — session-service: domain + adapter
- [ ] `internal/domain/session.go`: `FleetStatus` struct + `FleetInspector` interface
- [ ] `internal/allocator/agones.go`: `fleetGVR`; método `FleetStatus(ctx, fleetName)` (Get + NotFound tratado + parse dos campos numéricos)
- [ ] Unit test do adapter com fake dynamic client: Fleet existente, Fleet inexistente

## Fase 2 — session-service: transport + wiring
- [ ] `internal/transport/http_handler.go`: `AdminHandler` ganha `fleets domain.FleetInspector` + `clusterLabel string`; rota `GET /admin/fleet-status/{fleetName}`; handler cobre os 3 casos da spec (sem Agones / Agones sem a fleet / Agones com a fleet)
- [ ] Handler test cobrindo os 3 casos (EARS 2/3/4)
- [ ] `cmd/server/main.go`: lê `AGONES_CLUSTER_LABEL`; injeta o adapter Agones como `FleetInspector` só quando o allocator ativo é Agones
- [ ] `infra/docker/docker-compose.agones.yml` e `docker-compose.agones-pi5.yml`: `AGONES_CLUSTER_LABEL: local` / `pi5`

## Fase 3 — admin-bff
- [ ] `internal/domain`: `FleetStatus` (DTO local) + Port
- [ ] `internal/session/client.go`: método `FleetStatus(ctx, fleetName)`, com teste (server httptest)
- [ ] `internal/service/admin_service.go`: resolução `gameID → fleet` + chamada; caso "sem fleet cadastrada" sem round-trip; teste
- [ ] `internal/transport/http_handler.go`: rota `GET /games/{gameId}/fleet-status`; teste de handler (200, jogo sem fleet, session-service fora)

## Fase 4 — admin-ui
- [ ] `src/lib/fleetLocation.ts`: tipo + color/label helpers (offline/local/pi5/unknown)
- [ ] `src/components/FleetLocationBadge.tsx`
- [ ] `src/lib/api.ts`: tipo `FleetStatus` + `api.fleetStatus(gameId, signal)`
- [ ] `src/pages/Fleet.tsx`: dispara a chamada ao expandir a linha; nova seção "Onde está rodando" (badge + réplicas reais) ao lado da seção mockada existente
- [ ] `tsc --noEmit` + `npm run build` limpos

## Fase 5 — Verificação
- [ ] `go test ./services/{session-service,admin-bff}/... -race` limpo
- [ ] Smoke manual: backend com override `docker-compose.agones.yml` (k3d) → aba Fleet mostra "local (mac)" pro Capoeira Souls (fleet aplicada nesta mesma sessão de trabalho)
- [ ] Smoke manual: backend com override `docker-compose.agones-pi5.yml` → aba Fleet mostra "pi 5"
- [ ] Smoke manual: jogo sem fleet cadastrada → "offline" sem erro no console do browser
- [ ] Smoke manual: parar o session-service com a aba aberta → badge cai pra "desconhecido", página não crasha
