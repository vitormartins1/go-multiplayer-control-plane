# Tasks 050 — Polimento do form in-canvas + edição de regra

- [x] T1. Backend pronto (spec achievements/016): PUT + BFF passthrough +
      testes (feito no commit 4b0a1a6; 294 testes -race verdes).
- [x] T2. `api.ts` updateEventRule; rulesGraph com `editingRuleID` (nó da
      regra vira form no lugar) + ghosts adjacentes ao form (baricenter).
- [x] T3. Nós: hover-editar no RuleNode (scope game), RuleDraftNode com
      título criação/edição e inputs `.flow-input`; CSS (identidade dos
      inputs, largura fixa das fontes 190px, warning 9px).
- [x] T4. Página: DraftState.editingRuleID, startEdit, saveDraft POST/PUT,
      exclusividade criação×edição (um estado só), Esc.
- [x] T5. Alturas recalibradas com offsetHeight real (fonte 64/+15 drift,
      form 98+33/linha, ghost 73, type 64, ach 86, regra 69+22/cond) —
      gaps da coluna de métricas: 18/18/18 medidos.
- [x] T6. Build verde + smoke Playwright (hover→editar→form pré-carregado→
      salvar ≥5→≥6→reverter; ghost adjacente; Esc) + e2e do PUT via
      BFF+SQS (regra editada 100→40 muda o resultado do consumer; global
      404) + commit + rebuild containers.
