# Tasks: 059-fleet-tab
**Plan:** specs/059-fleet-tab/plan.md
**Data:** 2026-07-10

> Aba Fleet: expõe o campo `fleet` (ADR-0026, já real no game-registry, nunca
> propagado até o admin-bff/admin-ui) + esqueleto mockado da evolução futura
> (build/versão/health/promoção — nada disso existe hoje). Marcar `[x]`
> conforme implementa.

## Fase 1 — admin-bff: domain

- [x] `internal/domain/admin.go` — `Game` ganha `Fleet string \`json:"fleet"\``; `GameRegistry` ganha `SetFleet(ctx, id, fleet string) (*Game, error)` (doc comment perto de `RenameGame`)

## Fase 2 — admin-bff: game-registry client (adapter)

- [x] `internal/gameregistry/client.go` — `SetFleet` (`PUT /games/{id}/fleet`, corpo `{"fleet": fleet}`), mesmo tratamento de erro de `RenameGame` (upstream 4xx -> `UpstreamError`, indisponível -> `ErrGameRegistryUnavailable`)
- [x] `internal/gameregistry/client_test.go` — `TestSetFleet_RepassaAoRegistry` (httptest: sucesso 200, 404 -> UpstreamError(404), fora do ar -> unavailable)

## Fase 3 — admin-bff: service

- [x] `internal/service/admin_service.go` — `SetFleet(ctx, id, fleet string) (*domain.Game, error)` delega pro `GameRegistry`
- [x] `internal/service/admin_service_test.go` — `TestSetFleet_Delega` (fake `GameRegistry`)

## Fase 4 — admin-bff: transport

- [x] `internal/transport/http_handler.go` — `AdminService` (interface local) ganha `SetFleet`; rota `PUT /api/games/{gameId}/fleet`; handler `setFleet` (decode `{fleet}`, 400 se corpo inválido, `writeUpstream` em erro, 200 com o jogo atualizado)
- [x] `internal/transport/http_handler_test.go` — `fakeService.SetFleet`; testes 200/400/502-indisponível (mesmo padrão dos testes de `renameGame`/`deleteGame`)

## Fase 5 — admin-bff: fechamento

- [x] `go test ./services/admin-bff/... -race` verde (170 testes)
- [x] `go build ./...` verde
- [x] Doc de API — conferido `services/admin-bff/api/README.md`/`README.md`: nem `renameGame` nem `deleteGame` (endpoints irmãos já existentes) estão documentados ali, é um cookbook de curl não-exaustivo — não adicionei `setFleet` pra não criar uma inconsistência (um endpoint documentado sem os irmãos)

## Fase 6 — admin-ui: cliente HTTP

- [x] `src/lib/api.ts` — `Game` ganha `fleet?: string`; nova função `setFleet(gameId, fleet)` (`PUT /api/games/{gameId}/fleet`)

## Fase 7 — admin-ui: página Fleet (real + mock)

- [x] `src/pages/Fleet.tsx` (novo) — `mockFleetStatus(fleetName)` determinístico (sem chamada de rede); componente `Fleet` (lista de jogos via `useAsync(api.games)` + `Alert` de disclaimer fixo no topo); componente `FleetRow` (linha expansível, mesmo padrão de `GameSessions.tsx`)
- [x] `FleetRow` — jogo COM fleet: `Badge` com o nome; expandido mostra form real (`TextInput` + `Button Salvar` chamando `api.setFleet`) + seção mockada (tag atual, badge de health, `Select` de versões, botão "Promover" sempre `disabled` com `Tooltip` explicando que não está implementado)
- [x] `FleetRow` — jogo SEM fleet: texto "sem fleet configurada"; expandido mostra só o form real (sem seção mockada)
- [ ] Confirmar: nenhuma chamada de rede disparada pela seção mockada (nem pelo `Select` de versões, nem pelo botão "Promover") — verificação via Playwright na Fase 9

## Fase 8 — admin-ui: navegação

- [x] `src/App.tsx` — `const Fleet = lazy(...)`; entrada `{ to: "/fleet", label: "Fleet", icon: IconServer2 }` no array `NAV` (entre Analytics e Infrastructure); `<Route path="/fleet" element={<Fleet />} />`

## Fase 9 — verificação e fechamento

- [x] `tsc --noEmit` + `npm run build` limpos
- [x] Rebuild dos containers (`admin-bff`, `admin-ui`) via `docker compose -f infra/docker/docker-compose.yml build admin-bff admin-ui && ... up -d admin-bff admin-ui`
- [x] Smoke manual via Playwright:
  1. Nav mostra "Fleet"; navegar lá lista os 5 jogos ✓
  2. Packet Arena/Capoeira Souls (fleet real já cadastrada) aparecem com o nome real; expandir mostra a seção mockada com o disclaimer visível ✓
  3. Jogo sem fleet (ex. Sandbox) aparece com "sem fleet configurada", sem seção mockada ✓
  4. Editar o campo Fleet (Sandbox → "test-fleet-temp") e recarregar confirma persistência real; revertido de volta a "sem fleet configurada" pra não sujar os dados de demo ✓
  5. Clicar no `Select` de versões e no botão "Promover" (sempre disabled) dispara 0 requests de rede novas ✓
- [x] `docs/GO-LEARNINGS.md` — nada novo/não-óbvio aqui: é reuso mecânico do padrão já estabelecido (RenameGame espelhado ponta a ponta), sem conceito de Go novo
- [x] Adendo de ADR (0026/0029) **não** feito nesta fatia (fora de escopo — ver spec.md: só quando uma fase real futura for aprovada)
- [x] `go build ./...` + `go test ./... -race` no repo inteiro: 977 testes (+7 desta fatia), verde
