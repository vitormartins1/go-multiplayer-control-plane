# Spec: 080-event-rules-empty-canvas-hint

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 079 (rodada final da revisão aba por aba do console).

## Contexto

Última aba da revisão pedida pelo usuário (depois de Analytics, Sessions,
Leaderboards, Configurações, Players, Fleet). O canvas de Event Rules
(`GameEventRulesCanvas.tsx`, ~1500 linhas, kit de fluxo compartilhado com
Achievements/Pipeline) já passou por várias rodadas de refinamento
(specs 043 a 058) — ao contrário das outras abas, não é uma tabela, é um
editor de grafo (React Flow).

Comportamento deliberado desde a spec 055: sem nenhuma regra focada (e
fora de um rascunho de criação), o canvas fica vazio — mostrar o grafo
completo (15+ regras) de uma vez seria visualmente sobrecarregado.
Achado nesta revisão: essa decisão era correta, mas a execução deixava
o canvas **completamente em branco** na primeira visita — nenhuma pista
de que era preciso clicar numa regra na lista à esquerda. A única
menção a "selecione uma regra" existia escondida dentro do painel de
filtro (chips Facts/Modo/Stats), que só aparece expandido.

## Decisões

- `FlowCanvas` (componente compartilhado, spec 040+, usado por Event
  Rules/Achievements-prévia/Pipeline) ganha um prop opcional
  `emptyState?: ReactNode` — renderizado como um `Panel` do React Flow
  centralizado (`position="top-center"` + `top: 50%` /
  `translateY(-50%)`, sem interceptar clique), só quando
  `nodes.length === 0`. Opt-in: só quem passa o prop usa — Pipeline
  (sempre populado) e a prévia mini de Achievements (sempre focada) não
  precisam.
- `GameEventRulesCanvas.tsx` passa um hint compacto (ícone
  `IconHandClick` + "Selecione uma regra na lista à esquerda pra ver o
  grafo"), mesmo tom/tratamento visual (dimmed) do resto do console.
- **Mobile**: o dock (lista de regras) já ocupava a largura toda numa
  tela de 390px antes desta mudança — este editor é desktop-first por
  natureza (grafo com pan/zoom/drag não é um padrão de interação mobile
  viável). O hint novo, centralizado, ficava sobreposto à lista nessa
  largura — envolvido em `<Box visibleFrom="sm">` pra não aparecer
  onde colide.

## Critérios de Aceite

1. Canvas vazio (sem regra focada, sem rascunho) mostra o hint
   centralizado — antes era um retângulo preto sem nenhum texto.
2. Selecionar uma regra continua funcionando exatamente como antes (sem
   regressão no fluxo de foco/subgrafo/painel de detalhe).
3. Em ≥768px (sm) o hint aparece; abaixo disso, não — evita sobrepor o
   dock, que já ocupa a tela toda em mobile.
4. Sem erro de console.

## Fora de escopo

- Tornar o editor de grafo genuinamente utilizável em mobile (pan/zoom/
  drag num canvas nunca foi um padrão mobile-friendly; fora do que foi
  pedido).
- Qualquer mudança na lógica de foco/subgrafo (specs 055/056) — só a
  ausência de uma pista visual no estado vazio.
