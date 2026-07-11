# Spec: 020-player-by-id

**Serviço:** admin-bff
**Status:** Done
**Data:** 2026-07-08
**Parte de:** admin-ui/040-player-page-ux (handle no header do jogador).

## Contexto

O console precisa do handle de UM jogador na página `PlayerDetail`. Hoje o BFF só
resolve handles em lote via `handlesByID` (pagina o `GET /admin/players` inteiro).
Passthrough fino para o lookup direto do identity (identity/004).

## Comportamento

- THE SYSTEM SHALL expor `GET /api/players/{playerID}` devolvendo
  `{ "id": ..., "handle": ..., "createdAt": ... }` (mesma forma do item da listagem).
- IF o identity devolver 404, THEN o BFF devolve 404 (`{"error":"player_not_found"}`).
- IF o identity estiver indisponível, THEN 502 (padrão dos demais passthroughs).

## Casos de Teste / Verificação

- [ ] 200 com handle quando o identity responde
- [ ] 404 propagado; 502 com identity fora
