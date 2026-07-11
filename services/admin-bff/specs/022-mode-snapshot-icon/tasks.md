# Tasks: 022-mode-snapshot-icon
**Plan:** specs/022-mode-snapshot-icon/plan.md
**Data:** 2026-07-11

## Fase 1 — shared/modesnapshot + session-service
- [ ] `shared/modesnapshot/hash.go`: `Hash(archetype, topology string, params json.RawMessage) string` (corpo idêntico ao `profileHash` atual)
- [ ] `shared/modesnapshot/hash_test.go`: determinístico, entradas diferentes → hashes diferentes, params vazio não quebra
- [ ] `internal/registryclient/client.go` (session-service): remove `profileHash` local, `ModeProfile` chama `modesnapshot.Hash`
- [ ] `go test ./services/session-service/... -race` verde (nenhuma mudança de comportamento esperada)

## Fase 2 — admin-bff
- [ ] `internal/domain`: Port `GameRegistry` ganha `ModeSnapshotHashes(ctx, gameID) (map[string]string, error)`
- [ ] `internal/gameregistry/client.go`: implementa (decodifica `ListMultiplayerModes` cru em `[]struct{ModeID, Archetype, Topology string; Params json.RawMessage}`, aplica `modesnapshot.Hash` por item); teste com fixture de array JSON
- [ ] `internal/service/admin_service.go`: `Service.ModeSnapshotHashes(ctx, gameID)`; teste
- [ ] `internal/transport/http_handler.go`: rota `GET /api/games/{gameId}/multiplayer-profiles/snapshot-hashes`; teste (200 com mapa, jogo sem modos → `{}`, registry fora → 502)
- [ ] Teste de paridade: hash do endpoint == `profileHash` de uma sessão real aberta com a mesma config (fixture ou smoke manual)
- [ ] `go test ./services/admin-bff/... -race` verde

## Fase 3 — admin-ui: seed do ícone (baixo risco, primeiro)
- [ ] `src/lib/api.ts`: `api.modeSnapshotHashes(gameId, signal)`
- [ ] `src/pages/GameSessions.tsx`: `SessionAvatar seed={s.profileHash || s.sessionId}` (2 call sites)
- [ ] `src/pages/Rooms.tsx`: idem (2 call sites)
- [ ] Smoke visual: sessões do mesmo modo/config mostram o mesmo ícone; sessão de modo diferente mostra ícone diferente

## Fase 4 — admin-ui: tag de modo (ícone de arquétipo)
- [ ] `src/pages/GameSessions.tsx`: badge de modo troca `color="indigo"` por `leftSection={<ARCHETYPE_META[s.archetype]?.icon size={12} />}` + `color="gray"`; sem `archetype` → sem ícone, só texto
- [ ] `src/pages/Rooms.tsx`: idem

## Fase 5 — admin-ui: Configurações → Multiplayer
- [ ] `MultiplayerSettings.tsx`: busca `modeSnapshotHashes` junto do carregamento da tabela de modos
- [ ] Coluna nova (primeira, sem header ou "Ícone"): `SessionAvatar` seedado pelo hash do modo; estado de carregamento não bloqueia a tabela
- [ ] Coluna nova: hash em texto monoespaçado (mesmo padrão de `SessionMatchDetail.tsx`)

## Fase 6 — admin-ui: Rooms primeira coluna (capa + overlay)
- [ ] `Rooms.tsx`: primeira coluna nova com `GameCover` (variante pequena, ajustar tamanho se preciso) + `SessionAvatar` sobreposto (canto inferior-direito, `position: absolute`, aro/box-shadow igual ao `PlayersSummary`)
- [ ] Remove o `SessionAvatar` da coluna "Match" (fica só `matchId`)
- [ ] Confirma que a coluna "Jogo" (nome + tags) não mudou de conteúdo, só perdeu a responsabilidade da capa/ícone

## Fase 7 — Verificação
- [ ] `tsc --noEmit` + `npm run build` (admin-ui) limpos
- [ ] `go test ./services/{session-service,admin-bff}/... -race` limpo
- [ ] Smoke manual completo: mudar config de um modo em Configurações (ver ícone/hash mudar ali) → abrir sessão nova daquele modo → ícone em Rooms e em Sessions do jogo é o MESMO que já aparecia em Configurações antes de abrir
- [ ] Smoke manual: sessão legada sem `profileHash`/`archetype` (se existir no ambiente de dev) não quebra nenhuma das telas
