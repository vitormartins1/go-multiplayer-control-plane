# Spec: 010-game-rename

**Serviço:** admin-bff
**Status:** Draft
**Data:** 2026-07-04
**Parte de:** Console — Configurações do jogo (pares: `game-registry/005-game-rename`, `admin-ui/012-game-settings`)

## Contexto

Expõe o rename do game-registry (spec 005) para o painel. Passthrough fino, como o `CreateGame` já é.

## Comportamento

- WHEN `PUT /api/games/{gameId}` com `{name}` -> THE SYSTEM SHALL chamar `PUT /games/{id}` do game-registry (internal key) e devolver o jogo atualizado. Erro de validação (400) e não-encontrado (404) propagados (UpstreamError); registry fora -> 502.

## Contratos de Interface

- **Domain:** `GameRegistry.RenameGame(ctx, id, name) (*Game, error)`.
- **Adapter `gameregistry/client.go`:** `RenameGame` — `PUT /games/{id}` `{name}` (reusa `classify`).
- **Service:** `RenameGame(ctx, id, name)` (passthrough).
- **Transport:** `AdminService` +`RenameGame`; rota `PUT /api/games/{gameId}` (`writeUpstream`).

## Casos de Teste Obrigatórios

- [ ] `RenameGame` faz PUT com `{name}` e devolve o jogo (httptest do adapter)
- [ ] `PUT /api/games/{gameId}` -> 200; 400/404 propagados; registry fora -> 502

## Fora do escopo
- Delete de jogo (adiado).
