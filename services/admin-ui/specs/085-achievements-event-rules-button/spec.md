# Spec: 085-achievements-event-rules-button

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-13
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 084 (fatia pequena/aditiva: sem decisão técnica nova além do
prop `scrollZoom`, que só desacopla um comportamento já existente).

## Contexto

A aba Achievements de um jogo já tinha uma prévia read-only do subgrafo de
regras de um achievement (`RuleFocusMiniCanvas`), acessível clicando no
badge da coluna "Critério". O pedido: um segundo ponto de entrada para essa
mesma prévia — um botão na coluna de ações com o ícone de "Event Rules" —
sem duplicar a lógica. Junto, dois requisitos sobre o comportamento da
prévia em si: continuar dando pra mudar o zoom/navegar pelo minimapa, ser
readonly (já era), e **não travar o scroll da página** quando o mouse
passa por cima do canvas expandido.

## Decisões

### 1. Botão novo reaproveita `focusRow`

`GameAchievements.tsx`: `ActionIcon` com `IconGitBranch` (mesmo ícone da
aba "Event Rules" em `GameDetail.tsx`) adicionado à coluna de ações, antes
do botão "Ver progresso". `onClick` chama exatamente `expanded ?
focusRow(null) : focusRow(a.id)` — o mesmo par de funções que o clique no
badge de critério já usava. Não há novo estado, novo componente, nem novo
Modal: é o mesmo `RuleFocusMiniCanvas` inline (`Collapse` na própria
tabela), só com um segundo trigger.

### 2. `scrollZoom` — novo prop em `FlowCanvas`

Achado ao investigar o pedido de "não travar o scroll": o React Flow
sempre chama `preventDefault()` em qualquer wheel sobre o canvas quando
`zoomOnScroll`/`preventScrolling` estão ligados — e os dois já seguiam
`interactive` (`true` por padrão em `RuleFocusMiniCanvas`), então rolar a
página com o mouse sobre a prévia expandida sempre dava zoom no canvas em
vez de rolar a página. Um fix anterior (achado do Vitor, canvas do
Pipeline) já cobria isso para `interactive=false`, mas não para
`interactive=true` — que é exatamente o caso da prévia de achievements
(zoom pelo Controls e navegação pelo minimapa continuam desejados).

Solução: `zoomOnScroll`/`preventScrolling` passam a seguir um prop novo e
independente, `scrollZoom` (default = `interactive`, preservando o
comportamento de todos os outros canvas — Event Rules grande, Pipeline).
`RuleFocusMiniCanvas` passa `scrollZoom={false}` explicitamente:
`nodesDraggable`/`panOnDrag`/`zoomOnPinch`/`zoomOnDoubleClick`/`Controls`/
minimapa continuam ligados via `interactive=true` (não mudou), só o wheel
sobre o canvas para de ser capturado.

Como os dois pontos de entrada (badge de critério e botão novo) chamam o
mesmo `RuleFocusMiniCanvas`, o fix de scroll vale para os dois — não é
exclusivo do botão novo.

## Critérios de Aceite

1. `tsc --noEmit` limpo.
2. Botão "Ver regras do evento" (ícone `IconGitBranch`) aparece na coluna
   de ações de cada linha do catálogo — validado ao vivo (Playwright,
   jogo "Achievements Test", 66 achievements): clicar nele expande o
   mesmo canvas read-only que o badge de critério já abria.
3. Nós continuam não-arrastáveis/não-selecionáveis (readonly) — validado
   ao vivo: `RuleFocusMiniCanvas` já forçava isso antes desta mudança, sem
   regressão.
4. Rolar a página com o mouse sobre o canvas expandido rola a página
   normalmente, sem travar — validado ao vivo (Playwright: `scrollY`
   avançou de 0 para 400 após um `wheel` de 400px com o cursor sobre o
   `.react-flow`).
5. Zoom via botões do Controls e o minimapa continuam funcionando —
   validado ao vivo: clique no botão de zoom-in do Controls não lançou
   erro; `.react-flow__minimap` presente no DOM.
6. Sem erro de console.

## Fora de escopo

- Mudar o comportamento de scroll do canvas grande de Event Rules ou do
  Pipeline — `scrollZoom` default preserva o comportamento de sempre lá
  (só a prévia de achievements passa `false` explicitamente).
- Reposicionar o botão ou reordenar a coluna de ações além de inserir o
  novo item.
