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

*(Refinado 2026-07-08: modo e board viraram dois eixos **independentes**; o
`Select` de board foi substituído por um `SegmentedControl` com "Todas".)*

- THE SYSTEM SHALL oferecer dois `SegmentedControl` independentes junto aos
  controles existentes:
  - **Modo** ("Específica" | "Visão geral") — controla as **janelas**: a janela
    escolhida no switch Geral/Hoje/Semana/Mês (Específica) ou as 4 de uma vez
    (Visão geral, que oculta o switch de janela).
  - **Board** ("Todas" | Score | …) — controla os **rankings** exibidos. Sempre
    visível (nos dois modos); oculto apenas quando o jogo tem um board só.
- Matriz dos 4 estados:
  - **Específica + board específico** — comportamento original intacto: tabela
    única com paginação ("Carregar mais") e edição de score.
  - **Específica + Todas** — grid 2 colunas com um card por board (top 5), na
    janela escolhida; título do card = board, header da coluna = janela.
  - **Visão geral + board específico** — uma seção (divisor rotulado) com as
    4 janelas em grid 2 colunas; título do card = janela, header = board.
  - **Visão geral + Todas** — o panorama completo: uma seção por board, cada
    uma com as 4 janelas.
- Mini-tabelas (todos os estados exceto o primeiro): **top 5** (`limit=5`), sem
  paginação e sem o lápis de edição (ajuste de score é tarefa da tabela
  específica); **sempre 5 linhas** — posições sem dado preenchidas com "–"
  dimmed (janela vazia = tabela toda de "–"), mantendo o grid com altura
  uniforme. *(Refinado 2026-07-08: a 1ª versão mostrava um texto "Sem partidas
  no período." no lugar da tabela, quebrando a coesão.)*
- O botão Atualizar recarrega o que estiver visível (tabela única ou todas as
  mini-tabelas).
- As escolhas de modo/board não persistem entre visitas (estado local).

## Casos de Teste / Verificação

- [ ] Específica + board específico continua idêntico (janela, Carregar mais, editar)
- [ ] Específica + Todas: um card por board na janela escolhida
- [ ] Visão geral + board específico: 4 janelas só daquele board
- [ ] Visão geral + Todas: uma seção por board, 4 janelas por seção
- [ ] top 5 por card; posições sem dado viram "–" (sempre 5 linhas)
- [ ] Atualizar recarrega todas as mini-tabelas visíveis
- [ ] jogo sem stats (um board só) → switch de board oculto
- [ ] `npm run build` limpo

## Fora do escopo

- Edição de score na visão geral; paginação nas mini-tabelas; persistir o modo;
  cache/dedupe das 4×N chamadas (payloads pequenos, escala de admin).
