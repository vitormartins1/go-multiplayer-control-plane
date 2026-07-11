# Tasks: 060-rooms-grouped-sortable
**Plan:** specs/060-rooms-grouped-sortable/plan.md
**Data:** 2026-07-11

## Fase 1 — ordenação (sem agrupamento ainda)
- [x] `compareSessions(a, b, sortBy, sortDir, gamesById)`: comparador puro (jogo/arquétipo/modo/estado/players/tempos, regras da spec itens 12-15)
- [x] `SortableTh`: `<Table.Th>` clicável com seta de direção quando ativo
- [x] Coluna "Arquétipo" nova (badge ícone+rótulo, `ARCHETYPE_META`)
- [x] Coluna "Modo" separada da "Jogo" (badge só com `modeId`, sem ícone duplicado)
- [x] Cabeçalhos Jogo/Arquétipo/Modo/Estado/Players/Tempos usam `SortableTh` + `compareSessions`
- [x] Smoke visual: clicar cada cabeçalho ordena e inverte; seta reflete o estado — validado pelo usuário no browser (2026-07-11)

## Fase 2 — agrupamento por jogo
- [x] `RoomsTable` extraído (props: `rows`, `showGameColumn`, sort ativo) — reutilizado pela visão Geral e por cada seção
- [x] Agrupamento: `useMemo` agrupando `rows` (já filtradas por Abertas/Fechadas/Todas + Modo) por `gameId`, ordenado por contagem desc / nome alfabético no empate
- [x] Seção por jogo: cabeçalho com `GameCover` mini + nome + contagem; tabela interna sem a coluna "Jogo"
- [x] `SegmentedControl` Geral/Por jogo
- [x] Smoke visual: trocar de filtro (Abertas → Todas) reordena as seções se a contagem mudar — validado pelo usuário no browser (2026-07-11)

## Fase 3 — persistência
- [x] `useLocalStorage` (`@mantine/hooks`) pra `{view, sortBy, sortDir}` numa chave só
- [x] Smoke: escolher "Por jogo" + ordenar por Players, reload da página → mantém os dois — validado pelo usuário no browser (2026-07-11)

## Fase 4 — Verificação
- [x] `tsc --noEmit` + `npm run build` limpos
- [x] Smoke manual completo: todos os casos de teste do spec.md — validado pelo usuário no browser (2026-07-11)
- [x] Confirmação visual do usuário no browser — "beleza, validado" (2026-07-11); imagem Docker do admin-ui (`:8089`) rebuildada e recriada com o mesmo bundle validado
