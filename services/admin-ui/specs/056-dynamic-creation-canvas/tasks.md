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
