# Plan: 060-rooms-grouped-sortable
**Spec:** specs/060-rooms-grouped-sortable/spec.md
**Data:** 2026-07-11

## Abordagem técnica

Tudo client-side, sem chamada de rede nova — `Rooms.tsx` já tem `data` (sessões) e `games` (lista completa, adicionada na spec 022) na memória; agrupar e ordenar são só transformações de array (`useMemo`) sobre o que já existe.

Estrutura proposta:
- `useSortableRooms` (hook local, dentro do próprio arquivo ou em `lib/`) encapsula estado de sort (`sortBy`/`sortDir`) + a função de comparação (`compareSessions(a, b, sortBy, sortDir, gamesById)`), reaproveitada tanto pela tabela Geral quanto por cada seção da visão Por jogo (item 9 da spec: sort é uma configuração única).
- `useLocalStorage` do `@mantine/hooks` (já é dependência do projeto — confirmado, não precisa de hook novo) pra persistir `{view, sortBy, sortDir}` numa chave só (`rooms:view-prefs`).
- `RoomsTable` (componente extraído de dentro do JSX atual de `Rooms()`) recebe `rows` + `sortBy`/`sortDir`/`onSort` + uma flag `showGameColumn` (true na visão Geral, false dentro de uma seção) — evita duplicar o `<Table>`/`<Table.Thead>`/mapeamento de linhas entre os dois modos de visão.
- `RoomRow` ganha as duas colunas novas (Arquétipo, Modo) e perde o modeId/badge embutidos na coluna Jogo (viram suas próprias `<Table.Td>`).
- `SortableTh` (componente pequeno: `<Table.Th>` clicável com seta de direção quando ativo) — usado nos cabeçalhos ordenáveis, evita repetir a lógica de "está ativo? qual seta?" em 6 lugares.

## Arquivos a criar ou modificar

- `src/pages/Rooms.tsx` — grosso da mudança: estado de view/sort (via `useLocalStorage` do `@mantine/hooks`), agrupamento por jogo (`useMemo` agrupando `rows` por `gameId`, ordenado por contagem), `RoomsTable`/`SortableTh` extraídos, `RoomRow` com as 2 colunas novas
- Sem mudança em `api.ts`, `archetypeMeta.ts` (já tem tudo que precisa desde a spec 022), nem em nenhum serviço Go

## Compute e banco (ADR-0005)

N/A — zero mudança de backend.

## Dependências e libs

Nenhuma nova — `localStorage` é API do browser; ícones de seta (`IconChevronUp`/`IconChevronDown` ou equivalente) já vêm do `@tabler/icons-react`, já uma dependência.

## Decisões técnicas e trade-offs

- **Sort único (não por-seção)** — já na spec (item 9), repetido aqui porque é a decisão que mais simplifica a implementação: um `compareSessions` só, chamado com o mesmo `sortBy`/`sortDir` em qualquer contexto, em vez de um mapa de estados de sort por `gameId`.
- **Agrupamento client-side, sem endpoint novo** — os dados já estão todos na memória (`data.active`/`data.closed` + `games`); um endpoint "sessões agrupadas por jogo" no admin-bff seria trabalho de backend pra um cálculo que o frontend já faz de graça com o que tem.
- **`RoomsTable` extraído em componente** — decisão de refatoração, não de arquitetura: o card duplicado (tabela idêntica renderizada N vezes, uma por jogo, mais uma vez pra visão Geral) só se sustenta limpo se a tabela em si for uma função só, chamada com props diferentes (`rows`, `showGameColumn`) — sem isso, o componente cresce demais e a spec vira uma fonte de bugs de "esqueceu de replicar a mudança na visão Por jogo".
- **Desempate estável (item 11 da spec)** — não usa um segundo critério (ex.: "por Estado, depois por Tempos") de propósito; `Array.prototype.sort` do JS moderno já é estável (spec ECMA2019+), então basta NÃO adicionar um comparador secundário pra a ordem original da API se preservar em caso de igualdade.

## Ordem de implementação

1. `compareSessions` + `SortableTh` — lógica pura de ordenação, sem tocar no layout ainda
2. Colunas novas (Arquétipo, Modo separada) — no `RoomRow`, junto da troca dos cabeçalhos pra `SortableTh` (visão Geral, sem agrupamento ainda) — valida a ordenação e as colunas novas juntas, antes do agrupamento
3. Extrair `RoomsTable` (mesma tabela de hoje, agora reutilizável com `showGameColumn`) — refactor puro, sem mudança de comportamento visível
4. Agrupamento por jogo + `SegmentedControl` Geral/Por jogo
5. Persistência (view + sort) via `useLocalStorage` do `@mantine/hooks`

## Riscos e pontos de atenção

- **Contagem de sessões por seção depende do filtro atual** (Abertas/Fechadas/Todas) — mudar o filtro precisa re-agrupar e reordenar as seções (contagem muda), não só refiltrar as linhas dentro de uma seção fixa. Cobrir isso no teste manual: trocar de "Abertas" pra "Todas" e confirmar que a ordem das seções muda se a contagem mudar.
- **Sessão sem `gameId`** (deveria ser raro/inexistente, mas a API já trata como opcional em outros lugares do arquivo) — decidir onde ela cai na visão Por jogo: proposta é uma seção "Sem jogo" ao final (sem capa, ícone de jogo genérico) em vez de descartar a linha silenciosamente. Confirmar se esse caso realmente ocorre nos dados de hoje antes de superengenheirar — se nunca acontece na prática, um `console.warn` + omitir a linha é aceitável.
