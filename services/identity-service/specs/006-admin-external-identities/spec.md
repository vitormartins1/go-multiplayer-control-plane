# Spec: 006-admin-external-identities

**Serviço:** identity-service
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** `achievements/019-steam-mirror` precisa do SteamID do jogador

## Comportamento

- `GET /admin/players/{playerID}/external-identities` (internal key, como todo `/admin`) → `200`
  `{"identities": [{"provider": "steam", "externalId": "7656…"}]}`; lista vazia se o jogador não tiver vínculo;
  `404 player_not_found` se a conta não existir.
- Só leitura; não muda o login.

## Critérios de aceite

Transport (200 com vínculos, 200 vazio, 404), service (mock) e repositório (integração com Postgres real).
