# Spec: 017-multiplayer-modes-passthrough

**Serviço:** admin-bff
**Status:** Rascunho — aguardando aprovação
**Data:** 2026-07-07
**ADR:** 0024 · **Roadmap:** `multiplayer-modes-roadmap.md` — fatia **M0** (lado console, BFF)

## Contexto

Fatia **pequena/aditiva** (spec curta): o game-registry ganhou a coleção de modos (`game-registry/009`, na main); o console precisa alcançá-la. Como no perfil singular (spec `012`), o BFF é **passthrough cru** (`json.RawMessage` — não interpreta o corpo do modo; achado da BR4). O alias legado `/api/games/{gameId}/multiplayer-profile` continua existindo (a UI migra para a coleção nesta fatia, mas o alias não é removido — Aberturas do roadmap).

## Comportamento

- WHEN `GET /api/games/{gameId}/multiplayer-profiles` → repassa `GET /games/{id}/multiplayer-profiles` do game-registry e devolve o corpo cru (200/404).
- WHEN `PUT /api/games/{gameId}/multiplayer-profiles/{modeId}` com corpo cru → repassa e devolve o corpo/erro cru (200/400/404/409).
- WHEN `GET|DELETE /api/games/{gameId}/multiplayer-profiles/{modeId}` → repassa (200 · 204/404/409).
- Códigos de erro do upstream (`invalid_mode_id`, `mode_not_found`, `default_mode_protected`, `too_many_modes`, …) **fluem intactos** com o mesmo status (padrão do `internal/upstream`).
- Auth: mesmas regras das demais rotas `/api` (sessão admin); a internal key para o registry é do transport do client, como hoje.

## Critérios de aceite

- [ ] 4 rotas novas repassam método, corpo e status sem re-serializar o payload.
- [ ] Erro do upstream preserva status + código (ex.: DELETE default → 409 `default_mode_protected`).
- [ ] Alias legado `multiplayer-profile` intacto (testes existentes verdes).
- [ ] Testes unitários do client (`gameregistry`) e do handler para as 4 rotas.

## Fora do escopo

- Tipar o corpo do modo no BFF (segue `json.RawMessage`).
- Remoção do alias legado (Aberturas do roadmap, pós-M1).
