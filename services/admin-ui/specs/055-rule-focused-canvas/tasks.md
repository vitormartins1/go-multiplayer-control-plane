# Tasks 055 — canvas focado por regra + lista separada

- [x] T1. `rulesGraph.ts`: função `siblingRuleIds(edges, focusNodeIds)`
- [x] T2. `GameEventRulesCanvas.tsx`: props `focusRuleId`/`onFocusRule`; estado `showRelated`; `focusPath` (useMemo com `pathHighlight` + `siblingRuleIds`); filtro de `nodes`/`edges` finais (incl. `sectionLabel` órfão); caso vazio (`focusRuleId===null && !draft` → `[]`)
- [x] T3. `GameEventRulesCanvas.tsx`: `onNodeClick` chama `onFocusRule` quando o node clicado é `rule`/`ruleDraft`; `useEffect` sincroniza `selectedId` a partir de `focusRuleId`
- [x] T4. `GameEventRulesCanvas.tsx`: toggle "Regras relacionadas" na toolbar; estado vazio (`Selecione uma regra na lista`)
- [x] T5. `GameEventRules.tsx`: remove `SegmentedControl`/`view`; layout lista+canvas lado a lado (`Group`/`Box`); estado `focusRuleId`/`listFilter`
- [x] T6. `GameEventRules.tsx`: `<Table>` existente ganha `onClick` por linha + destaque visual da selecionada + campo de filtro de texto no header da lista
- [x] T7. `tsc --noEmit` + `npm run build` limpos (sem eslint configurado neste serviço)
- [x] T8. Smoke manual (Playwright) no jogo "Achievements Test" (13 regras): canvas vazio no load, foco por seleção (5 nós), toggle relacionadas (42 nós), deseleção volta ao vazio, os dois fluxos de criação (Modal da lista + ghost in-canvas) sem regressão, zero erros de console

## Achado corrigido durante a implementação (fora do plan.md original)

O canvas tem seu próprio seletor de arquétipo interno (`arch`/`archetype`), e o grafo só é construído para o arquétipo ativo (`archRules = rules.filter(r => r.archetype === arch)`) — mas a lista nova mostra regras de **todos** os arquétipos juntas. Selecionar uma regra de um arquétipo diferente do que estava ativo no canvas esvaziava o grafo inteiro (a regra focada nem existia no `graph` construído). Corrigido: o `useEffect` que sincroniza `focusRuleId` também troca `setArchetype` para o arquétipo da regra focada quando diverge.
