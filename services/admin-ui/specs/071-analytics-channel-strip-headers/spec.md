# Spec: 071-analytics-channel-strip-headers

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — sessão começou como
exploração visual (skill de frontend design), não passou pelo gate normal
de "spec antes de implementar". Fatia pequena/aditiva (só apresentação,
sem API/dado novo), mesmo caso do desvio já sinalizado da spec 068.

## Contexto

A aba Analytics de cada jogo (`GameAnalytics.tsx`) já tinha o layout
estático aprovado (specs 067/068: "ficou ótimo"). Faltava identidade
visual própria — cada seção de arquétipo era só um ícone pequeno + título,
sem nenhum elemento que amarrasse a página como um todo. Pedido do
usuário: melhorar a tela sem reabrir o que já foi aprovado (estrutura de
seções, cards, dashboards não-editáveis — ver `feedback-no-configurable-
dashboards` na memória).

Restrição deliberada herdada de decisões anteriores: **sem cor por
arquétipo** — já tentado e revertido (2026-07-10) porque colidia com as
cores do canvas de Event Rules na mesma tela. A paleta/tipografia
site-wide (`main.tsx`: Space Grotesk display, IBM Plex Sans corpo, IBM
Plex Mono dado, paleta slate-dark + accent "signal") também não foi
alterada — é usada pelo console inteiro, não só por esta página.

## Comportamento

- Cada seção (por arquétipo + a seção final cross-arquétipo, antes sem
  cabeçalho) ganha um cabeçalho "channel strip": badge do ícone num
  bezel quadrado, título, uma régua horizontal com marcações (visual de
  timebase de osciloscópio — referência literal ao nome "signal" já usado
  como accent do tema) e uma leitura monoespaçada à direita com o total
  daquele canal (o dado mais característico do arquétipo: partidas,
  eventos, participações, missões ou sessões).
- Divider entre seções removido — a régua do channel strip já cumpre esse
  papel; espaçamento vertical (`mt="lg"`) substitui.
- Seção final (antes sem título, "raw-stats"/"session-health"/
  "session-closed-daily") ganha o rótulo "Geral" + ícone `IconActivity`,
  consistente com as demais.
- Nenhuma mudança de dado, API ou estrutura de cards/grid — só o
  cabeçalho de seção.

## Contratos de Interface

Nenhum — puramente apresentação, front consome os mesmos campos de
`GameAnalyticsDetail` de sempre.

## Critérios de Aceite

1. Jogo com N arquétipos ativos -> N channel strips + 1 "Geral" (quando
   houver dado cross-arquétipo), cada um com leitura coerente com os
   dados exibidos abaixo.
2. Responsivo até mobile (390px): régua encolhe, ícone/título/leitura
   permanecem legíveis, sem quebra de layout.
3. Sem regressão nos cards/gráficos existentes (validado com 2 jogos de
   dados reais distintos — Neon Breach com 4 seções, Achievements Test
   com 2 + abandono real colorido).

## Fora de escopo

- Cor por arquétipo (decisão já revertida antes).
- Mudar a paleta/tipografia site-wide.
- Reabrir a estrutura de seções/cards (specs 067/068).
