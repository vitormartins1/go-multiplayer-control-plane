# Tasks: 022-mode-snapshot-icon
**Plan:** specs/022-mode-snapshot-icon/plan.md
**Data:** 2026-07-11

## Fase 1 — shared/modesnapshot + session-service ✅
- [x] `shared/modesnapshot/hash.go`: `Hash(archetype, topology string, params json.RawMessage) string` (corpo idêntico ao `profileHash` atual)
- [x] `shared/modesnapshot/hash_test.go`: determinístico, entradas diferentes → hashes diferentes, params vazio não quebra, sensível a bytes crus (ordem de chaves)
- [x] `internal/registryclient/client.go` (session-service): remove `profileHash` local, `ModeProfile` chama `modesnapshot.Hash`
- [x] `go test ./services/session-service/... -race` verde

## Fase 2 — admin-bff ✅
- [x] `internal/domain`: Port `GameRegistry` ganha `ModeSnapshotHashes(ctx, gameID) (map[string]string, error)`
- [x] `internal/gameregistry/client.go`: implementa (decodifica `ListMultiplayerModes` cru, aplica `modesnapshot.Hash` por item); testes (uma chamada só, mapa vazio, registry fora)
- [x] `internal/service/admin_service.go`: `Service.ModeSnapshotHashes`
- [x] `internal/transport/http_handler.go`: rota `GET /api/games/{gameId}/mode-snapshot-hashes` (fora de `/multiplayer-profiles/{modeId}` de propósito — achado durante a implementação: colidiria com um modo chamado literalmente "snapshot-hashes"); testes (200, mapa vazio, 502)
- [x] Teste de paridade: hash do endpoint == `profileHash` de sessão real aberta com a mesma config — confirmado ao vivo (`d9ee067e72e7b2a5` nos dois lados)
- [x] `go test ./services/admin-bff/... -race` verde

## Fase 3 — admin-ui: seed do ícone ✅
- [x] `src/lib/api.ts`: `api.modeSnapshotHashes(gameId, signal)`
- [x] `src/pages/GameSessions.tsx`: `SessionAvatar seed={s.profileHash || s.sessionId}` (2 call sites)
- [x] `src/pages/Rooms.tsx`: idem (2 call sites, incluindo a nova 1ª coluna da Fase 6)
- [x] Smoke: confirmado via paridade de hash (Fase 2) que o ícone reflete o snapshot real

## Fase 4 — admin-ui: tag de modo (ícone de arquétipo) ✅
- [x] `src/lib/archetypeMeta.ts`: helper `archetypeMetaOf` (indexação segura por string arbitrária — `archetype` de sessão não é o union tipado)
- [x] `src/pages/GameSessions.tsx`: badge de modo troca `color="indigo"` por ícone + `color="gray"`; sem `archetype` → sem ícone, só texto
- [x] `src/pages/Rooms.tsx`: idem

## Fase 5 — admin-ui: Configurações → Multiplayer ✅
- [x] `MultiplayerSettings.tsx`: busca `modeSnapshotHashes` em paralelo (efeito próprio, não bloqueia a tabela de modos)
- [x] Coluna nova (primeira, sem header): `SessionAvatar` seedado pelo hash; `Loader` enquanto carrega, "—" se faltar
- [x] Coluna nova "Snapshot": hash em texto monoespaçado

## Fase 6 — admin-ui: Rooms primeira coluna (capa + overlay) ✅
- [x] `Rooms.tsx`: busca `api.games()` uma vez (mapa por id, mesmo espírito do `Fleet.tsx`) — achado durante a implementação: Rooms não carregava a lista de jogos antes, só nome/id da sessão
- [x] Primeira coluna: `GameCover` (variante mini, 28x28) + `SessionAvatar` sobreposto (canto inferior-direito, `position: absolute`, `box-shadow` de aro igual ao `PlayersSummary`); fallback pro ícone sozinho se o jogo não for encontrado no mapa
- [x] Removido o `SessionAvatar` da coluna "Match" (fica só `matchId`)
- [x] Coluna "Jogo" mantida sem mudança de conteúdo

## Fase 7 — Verificação ✅
- [x] `tsc --noEmit` + `npm run build` (admin-ui) limpos
- [x] `go test ./... -race` do monorepo inteiro limpo (1011 testes, 86 pacotes)
- [x] Smoke ao vivo: `GET /api/games/{id}/mode-snapshot-hashes` (Capoeira Souls, 8 modos) → abriu sessão real do modo `default` → `profileHash` da sessão idêntico ao hash do endpoint pra aquele modo
- [x] Verificação visual no browser: **pendente do usuário** (sem ferramenta de automação de navegador nesta sessão)
