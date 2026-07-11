# Tasks: 060-rooms-grouped-sortable
**Plan:** specs/060-rooms-grouped-sortable/plan.md
**Data:** 2026-07-11

## Fase 1 — ordenação (sem agrupamento ainda)
- [ ] `compareSessions(a, b, sortBy, sortDir, gamesById)`: comparador puro (jogo/arquétipo/modo/estado/players/tempos, regras da spec itens 12-15)
- [ ] `SortableTh`: `<Table.Th>` clicável com seta de direção quando ativo
- [ ] Coluna "Arquétipo" nova (badge ícone+rótulo, `ARCHETYPE_META`)
- [ ] Coluna "Modo" separada da "Jogo" (badge só com `modeId`, sem ícone duplicado)
- [ ] Cabeçalhos Jogo/Arquétipo/Modo/Estado/Players/Tempos usam `SortableTh` + `compareSessions`
- [ ] Smoke visual: clicar cada cabeçalho ordena e inverte; seta reflete o estado

## Fase 2 — agrupamento por jogo
- [ ] `RoomsTable` extraído (props: `rows`, `showGameColumn`, sort ativo) — reutilizado pela visão Geral e por cada seção
- [ ] Agrupamento: `useMemo` agrupando `rows` (já filtradas por Abertas/Fechadas/Todas + Modo) por `gameId`, ordenado por contagem desc / nome alfabético no empate
- [ ] Seção por jogo: cabeçalho com `GameCover` mini + nome + contagem; tabela interna sem a coluna "Jogo"
- [ ] `SegmentedControl` Geral/Por jogo
- [ ] Smoke visual: trocar de filtro (Abertas → Todas) reordena as seções se a contagem mudar

## Fase 3 — persistência
- [ ] `useLocalStorage` (`@mantine/hooks`) pra `{view, sortBy, sortDir}` numa chave só
- [ ] Smoke: escolher "Por jogo" + ordenar por Players, reload da página → mantém os dois

## Fase 4 — Verificação
- [ ] `tsc --noEmit` + `npm run build` limpos
- [ ] Smoke manual completo: todos os casos de teste do spec.md
- [ ] Confirmação visual do usuário no browser
