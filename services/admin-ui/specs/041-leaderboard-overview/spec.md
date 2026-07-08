# Spec: 041-leaderboard-overview

**Serviço:** admin-ui
**Status:** Aprovado (2026-07-08) — comportamento definido pelo usuário; detalhes de UX delegados
**Data:** 2026-07-08
**Parte de:** lote de UX do console. Client-only: usa o `GET /leaderboard` existente (spec 039/019).

## Contexto

A aba Leaderboards mostra UMA tabela por vez: o admin alterna janela (Geral/Hoje/
Semana/Mês) no `SegmentedControl` e board (Score, stats, Vitórias…) no `Select`.
Para ter o panorama do jogo é preciso clicar 4×N vezes. A visão geral mostra tudo
de uma vez, com o modo atual preservado para investigação pontual (e edição).

## Comportamento

- THE SYSTEM SHALL oferecer um `SegmentedControl` de modo de exibição
  ("Específica" | "Visão geral") junto aos controles existentes. Default:
  "Específica" (comportamento atual, intacto — janela, board, paginação, edição).
- No modo **"Visão geral"**:
  - THE SYSTEM SHALL exibir uma **seção por board** (todos os `boardOptions` do
    jogo), com divisor rotulado com o nome do board — sem o `Select`;
    com um único board (jogo sem stats), o divisor ainda identifica "Score".
  - Cada seção dispõe as **4 janelas** (Geral, Hoje, Semana, Mês) em grid de
    **2 colunas** (1 em telas menores), cada uma como mini-tabela com título.
  - Cada mini-tabela mostra o **top 5** (`limit=5`), sem paginação e sem o lápis
    de edição (ajuste de score é tarefa do modo Específica).
  - Toda mini-tabela tem **sempre 5 linhas**: as posições sem dado são
    preenchidas com "–" dimmed (janela vazia = tabela toda de "–"), mantendo o
    grid com altura uniforme. *(Refinado 2026-07-08: a 1ª versão mostrava um
    texto "Sem partidas no período." no lugar da tabela, quebrando a coesão.)*
  - O botão Atualizar recarrega todas as mini-tabelas.
  - Os controles de janela (`SegmentedControl` Geral/Hoje/…) e o `Select` de
    board ficam ocultos (não se aplicam).
- A escolha do modo não persiste entre visitas (estado local).

## Casos de Teste / Verificação

- [ ] modo Específica continua idêntico (janela, board, Carregar mais, editar)
- [ ] Visão geral: uma seção por board na ordem do seletor, 4 janelas por seção
- [ ] top 5 por janela; janela vazia mostra o texto dimmed
- [ ] Atualizar na visão geral recarrega todas as tabelas
- [ ] jogo sem stats (só Score) → uma seção única
- [ ] `npm run build` limpo

## Fora do escopo

- Edição de score na visão geral; paginação nas mini-tabelas; persistir o modo;
  cache/dedupe das 4×N chamadas (payloads pequenos, escala de admin).
