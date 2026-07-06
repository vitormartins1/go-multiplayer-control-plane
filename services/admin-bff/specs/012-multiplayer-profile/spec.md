# Spec: 012-multiplayer-profile

**Serviço:** admin-bff
**Status:** Draft
**Data:** 2026-07-06
**Parte de:** B1 do roadmap `queue_arena` (ADR-0023). Pares: `game-registry/006-multiplayer-profile` (backend, feito), `admin-ui/015-multiplayer-profile` (UI).

## Contexto

Expõe o perfil de multiplayer do game-registry (spec 006: `GET`/`PUT /games/{id}/multiplayer-profile`) para o painel. Passthrough fino, como `RenameGame` (spec 010). O BFF **não interpreta** a forma do perfil — repassa o corpo cru nos dois sentidos, para não acoplar-se à evolução do modelo (novos arquétipos). Erros de validação/estado do game-registry propagam via `UpstreamError` (código estável já emitido pelo backend: `invalid_profile_params`, `unsupported_archetype`, `unsupported_topology`, `not_found`, `profile_not_found`).

## Comportamento

- WHEN `GET /api/games/{gameId}/multiplayer-profile` -> THE SYSTEM SHALL chamar `GET /games/{id}/multiplayer-profile` do game-registry e devolver o perfil (`200`). Jogo sem perfil -> `404 profile_not_found` propagado; jogo inexistente -> `404 not_found`; registry fora -> `502`.
- WHEN `PUT /api/games/{gameId}/multiplayer-profile` com o corpo do perfil -> THE SYSTEM SHALL chamar `PUT /games/{id}/multiplayer-profile` (internal key, ADR-0018) e devolver o perfil persistido (`200`). Validação inválida -> `400 invalid_profile_params`/`unsupported_*` propagado; jogo inexistente -> `404`; registry fora -> `502`.

## Contratos de Interface

- **Domain (`GameRegistry`):**
  - `GetMultiplayerProfile(ctx, gameID string) (json.RawMessage, error)`
  - `SetMultiplayerProfile(ctx, gameID string, profile json.RawMessage) (json.RawMessage, error)`
- **Adapter `gameregistry/client.go`:** ambos via `upstream.Do` (reusa a classificação 2xx/4xx). Corpo cru (`io.ReadAll`) nos dois sentidos.
- **Service:** `GetMultiplayerProfile`/`SetMultiplayerProfile` (passthrough).
- **Transport:** `AdminService` +2 métodos; rotas `GET`/`PUT /api/games/{gameId}/multiplayer-profile` (`writeUpstream`). O PUT repassa o corpo cru recebido do cliente (sem `DisallowUnknownFields` — o game-registry é a verdade da validação).

## Casos de Teste Obrigatórios

- [ ] `GetMultiplayerProfile` faz GET e devolve o corpo cru; 404 -> `UpstreamError{404}` (httptest do adapter)
- [ ] `SetMultiplayerProfile` faz PUT com o corpo cru e devolve o perfil; 400 -> `UpstreamError{400, invalid_profile_params}`
- [ ] `GET /api/games/{gameId}/multiplayer-profile` -> 200; 404 propagado; registry fora -> 502
- [ ] `PUT /api/games/{gameId}/multiplayer-profile` -> 200; 400/404 propagados; registry fora -> 502

## Fora do escopo

- Interpretar/validar o perfil no BFF (o game-registry é a verdade).
- Outros arquétipos além de `queue_arena` (o passthrough cru já os acomoda sem mudança).
