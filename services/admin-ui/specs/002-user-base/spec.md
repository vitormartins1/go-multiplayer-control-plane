# Spec: 002-user-base

**App:** admin-ui (front-end)
**Status:** Approved
**Data:** 2026-06-29

## Contexto

Iteração 2 (ADR-0016): preenche a página **User base** do painel, que na `001-control-room` era só RED + placeholder "Iteração 2". Agora lista os usuários cadastrados, consumindo `GET /api/players` do `admin-bff` (que repassa o `/admin/players` do identity). SDD adaptado (spec = UX), como a 001.

## Comportamento (UX)

- WHEN a página **User base** abre -> além de saúde + RED, mostra a contagem total de usuários e uma **tabela** (handle, id abreviado, criado em) da primeira página.
- THE APP SHALL paginar com **Anterior/Próxima** (página de 50), exibindo "início–fim de total"; desabilita Anterior no começo e Próxima no fim.
- IF não houver usuários -> linha "Nenhum usuário cadastrado".
- IF `GET /api/players` responder 502 (identity fora) -> mensagem de erro no painel (não quebra a página).
- THE APP SHALL remover o item "listar usuários" do bloco "Iteração 2" do domínio identity (agora implementado).

## O que consome
`GET /api/players?limit=&offset=` -> `{ total, players: [{id, handle, createdAt}] }` (admin-bff/003).

## Critérios de aceitação
- [ ] User base mostra total + tabela paginada (handle/id/createdAt)
- [ ] Anterior/Próxima paginam; limites desabilitam os botões
- [ ] vazio -> mensagem; 502 -> erro sem quebrar
- [ ] o placeholder "listar usuários" sai do bloco Iteração 2 do identity

## Fora do escopo
- Busca/filtro por handle, detalhe de usuário, ações (banir/editar) — incremental/iteração futura
- Auth — interno/local no MVP
