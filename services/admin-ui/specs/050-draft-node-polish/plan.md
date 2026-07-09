# Plan 050 — Polimento do form in-canvas + edição de regra

## Decisões técnicas

1. **Um único form-node pra criar E editar.** O `RuleDraftNode` já é o form;
   a edição REUTILIZA o mesmo componente e o mesmo `DraftState`, com um campo
   novo `editingRuleID: string | null`. `null` = criação (nó `r:__draft__` no
   topo); setado = o nó da própria regra troca `type: "rule"` →
   `type: "ruleDraft"` no build (mesma posição na coluna, cresce no lugar).
   Header muda ("Nova regra" / "Editar regra") via flag em `RuleDraftData`.
2. **Estado inicial da edição** derivado da regra: `rows` a partir de
   `Object.entries(match)` (`condOp`/`condRaw` de matchCond.ts já extraem
   op+valor em string), `linked` = eventTypes existentes, `news` vazio.
   Salvar chama `api.updateEventRule` (PUT novo no api.ts); criação continua
   no POST. Draft e edição são o MESMO estado → mutuamente exclusivos de
   graça.
3. **Hover "editar"**: no `RuleNode` (scope game), o badge "editável" vira
   botão no hover (CSS: swap por opacity dentro do header; `nodrag`).
   Callback `onEdit` injetado via node.data no decorate da página (mesmo
   padrão dos callbacks do draft).
4. **Ghost adjacente ao rascunho**: incluir os ghosts no mapa de vizinhos do
   baricentro da coluna de types com o índice do nó em edição (draft OU regra
   em edição) — hoje eles não têm entrada e caem no fim
   (`MAX_SAFE_INTEGER`).
5. **Inputs com identidade do card**: classe `.flow-input` aplicada via
   `classNames` do Mantine (Autocomplete/inputs) — fundo `#131a24`, borda
   `#2a3340`, mono 11px, foco azul discreto; botões do rodapé
   `variant="subtle"` compactos. Sem trocar de lib.
6. **Cards de fonte uniformes**: `.flow-card--src { width: 190px }` fixa
   (fact/mode/stat), warning de drift em 9px com `line-height` apertado;
   altura estimada do drift recalibrada medindo `boundingBox` real no smoke
   (critério: gaps da coluna de métricas iguais ±2px).

## Arquivos

- `src/lib/api.ts` — `updateEventRule` (PUT).
- `src/lib/matchCond.ts` — (já tem condOp/condRaw; sem mudança prevista).
- `src/lib/rulesGraph.ts` — `editingRuleID` no DraftGraphInput; nó da regra
  vira ruleDraft; ghosts no baricentro de types; alturas recalibradas.
- `src/components/flow/nodes.tsx` — RuleNode com hover-editar; RuleDraftNode
  com título dinâmico e classes `.flow-input`; EventTypeDraftNode idem.
- `src/components/flow/flow.css` — `.flow-input`, largura fixa das fontes,
  warning 9px, hover do editar.
- `src/pages/GameEventRulesCanvas.tsx` — DraftState.editingRuleID,
  startEdit(rule), saveDraft bifurca POST/PUT, onEdit no decorate.

## Riscos

- Alturas estimadas vs reais (gotcha conhecido) — mitigado medindo no smoke.
- Edição de regra com condição inválida vinda do banco (op desconhecido):
  `condOp`/`condRaw` degradam pra eq/valor bruto — aceitável (validação
  estrita no salvar impede persistir pior do que estava).
