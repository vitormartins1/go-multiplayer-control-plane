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

## Follow-up: repack de posições no subgrafo filtrado (2026-07-10)

Achado do Vitor testando na prática: os achievements de uma regra focada apareciam isolados uns dos outros mesmo já filtrados — `buildRulesGraph` posiciona pensando no arquétipo **inteiro**; filtrar nodes sem recalcular Y deixa vãos enormes onde ficavam os nodes escondidos. Corrigido com `repackVisible(nodes)` (`rulesGraph.ts`) — reempacota cada coluna do subconjunto visível do zero (mesma lógica de `stackColumn`, alturas reais por tipo de node via `heightOf`), preservando a ordem relativa e recentralizando pela coluna mais alta. Só roda no ramo `focusPath` ativo (nunca com draft — nesse caso o filtro já está desligado, grafo completo). Confirmado visualmente via Playwright/screenshot: gaps entre nodes consecutivos caíram de potencialmente centenas de px pra ~10–25px (máximo observado 78px).

## Follow-up: "regras relacionadas" exige mesmo VALOR, não só mesma chave (2026-07-10)

Achado do Vitor: `siblingRuleIds` original considerava "relacionada" qualquer regra que tocasse a mesma SOURCE (fact/mode/stat) da regra focada, mesmo com valores diferentes (ex.: `role=coop` e `role=invader` apareciam relacionadas só por usarem a chave `role`). Corrigido: agora exige o mesmo LABEL de condição exato (ex.: `= coop`) — compartilhar a chave sem compartilhar o valor não conta mais. Event type continua bastando compartilhar (é um identificador discreto, não tem "valor" pra comparar).

Segundo achado, testando com dado real: mesmo com valor exato, uma condição muito genérica (`outcome=SUCCESS`, usada por quase toda regra do jogo de teste) sozinha ainda gera bastante fan-out. Solução do Vitor: trocar o switch único por um `Chip.Group` (Facts/Modo/Stats) controlando `allowedKinds` separadamente — dá pra restringir "relacionadas" a só stats, por exemplo, evitando o ruído de facts genéricos. Nenhum chip selecionado = mecanismo inteiro desligado (mesmo efeito do switch em "off" antes). `siblingRuleIds` ganhou o parâmetro `allowedKinds: Set<SourceKind>` e passou a receber `nodes` também (precisa do `kind` de cada source pra filtrar).

Confirmado via Playwright no jogo "Achievements Test": regra focada com 11 nós base — "só Facts" sobe pra 39 (confirma que `outcome=SUCCESS` é o ruído), "só Stats"/"só Modo" ficam em 11 (sem siblings por esses kinds nessa regra), nenhum chip selecionado volta a 11.
