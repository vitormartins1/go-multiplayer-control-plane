# Tasks 044 — Canvas UX v2

- [x] T1. `rulesGraph.ts` — param `draft` (pseudo-regra `__draft__` no layout),
      marcação `is-draft` em nós/edges ghost.
- [x] T2. Kit flow — `FlowCanvas` com prop `overlay` (Panel top-right);
      `RuleNode` com badge/estilo rascunho; CSS `is-draft`.
- [x] T3. `GameEventRulesCanvas.tsx` — toolbar (arquétipo + Nova regra), painel
      flutuante fechável, drawer de baixo com form + chips de facts, draft ao
      vivo (click/drag adiciona), submit/cancel sem resíduo.
- [x] T4. Build verde + smoke visual (screenshots: layout novo e rascunho ligado
      ao vivo). Fixes do smoke: `flex:1` do wrapper colapsava a altura dentro
      de Stack (removido — CSS manda); `AutoFit`/`height` no FlowCanvas pra
      re-enquadrar quando o grafo muda e encolher o canvas com o drawer aberto.
