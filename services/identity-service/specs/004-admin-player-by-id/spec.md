# Spec: 004-admin-player-by-id

**Serviço:** identity-service
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Parte de:** admin-ui/040-player-page-ux (handle no header do jogador), via admin-bff/020.

## Contexto

A superfície admin (003) só lista jogadores paginados. O console precisa de lookup
por id para exibir o handle de um jogador específico sem paginar a base toda.

## Comportamento

- THE SYSTEM SHALL expor `GET /admin/players/{playerID}` devolvendo o mesmo shape do
  item da listagem admin (`id`, `handle`, `createdAt`).
- IF o id não existir, THEN 404 (`{"error":"player_not_found"}`).
- Mesma proteção/estilo da superfície admin existente (003) — sem auth de player.

## Casos de Teste / Verificação

- [ ] 200 com o jogador quando existe
- [ ] 404 quando não existe
- [ ] repositório: lookup por id com `ctx`; erro wrappado no padrão do serviço
