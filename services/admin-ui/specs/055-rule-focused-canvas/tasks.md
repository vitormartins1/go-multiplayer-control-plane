# Tasks 055 — canvas focado por regra + lista separada

- [ ] T1. `rulesGraph.ts`: função `siblingRuleIds(edges, focusNodeIds)`
- [ ] T2. `GameEventRulesCanvas.tsx`: props `focusRuleId`/`onFocusRule`; estado `showRelated`; `focusPath` (useMemo com `pathHighlight` + `siblingRuleIds`); filtro de `nodes`/`edges` finais (incl. `sectionLabel` órfão); caso vazio (`focusRuleId===null && !draft` → `[]`)
- [ ] T3. `GameEventRulesCanvas.tsx`: `onNodeClick` chama `onFocusRule` quando o node clicado é `rule`/`ruleDraft`; `useEffect` sincroniza `selectedId` a partir de `focusRuleId`
- [ ] T4. `GameEventRulesCanvas.tsx`: toggle "Regras relacionadas" na toolbar; estado vazio (`Selecione uma regra na lista`)
- [ ] T5. `GameEventRules.tsx`: remove `SegmentedControl`/`view`; layout lista+canvas lado a lado (`Group`/`Box`); estado `focusRuleId`/`listFilter`
- [ ] T6. `GameEventRules.tsx`: `<Table>` existente ganha `onClick` por linha + destaque visual da selecionada + campo de filtro de texto no header da lista
- [ ] T7. `tsc --noEmit` + `eslint` limpos; `npm run build` verde
- [ ] T8. Smoke manual (Playwright) no jogo "Achievements Test" (66 regras): canvas vazio no load, foco por seleção, edição salva, toggle relacionadas, deseleção, os dois fluxos de criação sem regressão
