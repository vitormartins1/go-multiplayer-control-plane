# Tasks 056 — criação dinâmica + foco na edição

- [x] T1. `focusPath`: passa a usar `effectiveFocusId` (`draft ? draft.editingRuleID : focusRuleId`) em vez de desligar sempre que `draft` existe
- [x] T2. Extrair `applyFocusFilter(path)` (repete o filtro+repack já usado na spec 055) dentro do `useMemo` de nodes/edges
- [x] T3. Novo ramo de criação pura (`draft && !draft.editingRuleID`): só draft rule + ghost(s) de tipo + sources referenciadas pelas chaves válidas de `draft.rows`
- [x] T4. Reconectar os 3 ramos (criação pura / foco via `applyFocusFilter` / vazio) no `useMemo` principal
- [x] T5. Importar `DRAFT_RULE_ID` de `rulesGraph.ts`; ajustar `fitKey` pra reagir à contagem de nós visíveis (`nodes.length`) em vez de tentar rastrear chaves referenciadas separadamente
- [x] T6. `tsc --noEmit` + `npm run build` limpos
- [x] T7. Smoke manual (Playwright): nova regra vazia (2 nós), source aparece ao selecionar no autocomplete (3 nós), edição focada (9→10 nós, não o grafo inteiro), zero erros de console

## Nota da verificação

As duas primeiras rodadas do smoke test "falharam" por erros do próprio script, não da implementação: (1) `Escape` pra fechar o dropdown do Autocomplete na verdade cancela o rascunho inteiro (comportamento correto e já existente, spec 048) — troquei por selecionar a sugestão via clique; (2) o seletor `.mantine-Combobox-option` estava errado — a classe real é `.mantine-Autocomplete-option`. Depois de corrigir o script, todos os comportamentos da spec confirmaram certos.

## Follow-up: repack também no ramo de criação pura (2026-07-10)

O plan.md original decidiu deliberadamente **não** repacotar o ramo de criação ("são poucas sources, repacotar traria pouco ganho") — errado na prática: o Vitor testou e as sources apareciam nas coordenadas Y da coluna inteira, não organizadas com base só no que está na tela. Corrigido aplicando `repackVisible(filtered)` também nesse ramo (mesma função já usada pela spec 055/pelo foco) — confirmado visualmente com 3 sources (event/role/stat.kills) ficando empacotadas e centralizadas junto da regra e do ghost de tipo.

## Follow-up 2: ghost espúrio na edição + colisão horizontal (2026-07-10)

Dois achados do Vitor testando os fluxos de criar/editar:

1. **Ghost de tipo novo abrindo sozinho ao editar.** `startEdit` chamava `news: newGhost()` incondicionalmente — abria uma caixinha "event type novo" vazia mesmo sem o usuário pedir, já que linkar/desvincular tipos existentes já funciona clicando no node (`linked: [...rule.eventTypes]`). Corrigido: `news: []` ao editar. Fica pendente (deliberadamente, "depois a gente pensa nisso") uma forma explícita de adicionar um tipo novo durante a edição — hoje só dá pra fazer isso na criação.
2. **Caixinha de edição colava na coluna de event types.** `.flow-card--editing` tem 340px de largura (CSS) contra os 260px máximos de uma regra normal — o gap entre as colunas (`COL_X[1]`→`COL_X[2]` = 340px) já era calibrado só pro card normal, então a caixinha de edição/criação encostava direto na próxima coluna sem respiro. Corrigido com `DRAFT_COL_SHIFT = 80` (`rulesGraph.ts`): quando há draft ativo, as colunas de event types e achievements deslocam 80px pra direita, restaurando o mesmo respiro que existe fora do draft. Vale tanto pra criar quanto pra editar (mesmo estilo de card nos dois fluxos).

## Follow-up 3: colunas não acompanhavam o crescimento real da caixinha (2026-07-10)

Pedido do Vitor: o node "event type novo" (e as sources à esquerda) devem ficar sempre alinhados ao meio da caixinha de regra, inclusive conforme ela cresce/encolhe ao adicionar/remover condições — reposicionamento dinâmico, nos dois fluxos (criar e editar).

Causa raiz: `heightOf` (a função que `repackVisible` usa pra saber quanto espaço cada node ocupa) não tinha um caso pra `ruleDraft`/`eventTypeDraft` — caía no `default` (86px fixo), enquanto a altura REAL do card (`draftHeight(rows)`) cresce 32px por linha de condição. Como a estimativa nunca mudava, o reposicionamento (que já existia desde os follow-ups anteriores) não tinha como acompanhar o crescimento de verdade.

Corrigido: `repackVisible` ganhou um parâmetro opcional `heightOverrides: Map<string, number>`; o canvas monta esse mapa com a altura real do node em edição/criação (`draftHeight(draft.rows.length)`, mesma fórmula do build) e passa pros dois call-sites (`applyFocusFilter`, usado na edição, e o ramo de criação pura). `draftHeight` foi exportada de `rulesGraph.ts` pra não duplicar a fórmula. `heightOf` também ganhou os casos `ruleDraft`/`eventTypeDraft` explícitos (fallback pra 1 linha/71px quando não há override, em vez do `default` errado de 86px).

Confirmado via Playwright: testando 1, 2 e 3 condições na criação, a diferença entre o centro vertical da regra e do ghost de tipo ficou em 0-1px em todos os casos (antes, cresceria proporcional às linhas adicionadas). Mesma verificação na edição.
