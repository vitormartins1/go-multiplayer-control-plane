# Tasks: 002-user-base
**Spec:** specs/002-user-base/spec.md
**Data:** 2026-06-29

> Increment do front: página User base lista usuários (via admin-bff/003).

- [x] `src/lib/api.ts` — tipos `Player`/`PlayerPage` + chamada `players(limit, offset)`
- [x] `src/components/UsersPanel.tsx` — total + tabela paginada (Anterior/Próxima); vazio/erro/loading
- [x] `src/lib/domains.ts` — identity ganha `hasUsers: true`; remove o item "Iteração 2" de listar usuários
- [x] `src/components/DomainPage.tsx` — renderiza `UsersPanel` quando `hasUsers`
- [x] `npm run build` (tsc + vite) OK; validado no Docker (8089) — `/api/players` via nginx -> BFF -> identity
