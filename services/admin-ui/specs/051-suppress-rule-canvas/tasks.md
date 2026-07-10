# Tasks 051 — Suprimir/restaurar regra global no canvas (V2b)

- [ ] T1. Backend pronto (spec achievements/017): endpoints
      suppress/unsuppress + `suppressed` no DTO, containers rebuildados.
- [ ] T2. `api.ts`: `EventRule.suppressed`, `suppressEventRule`,
      `unsuppressEventRule`.
- [ ] T3. `rulesGraph.ts`: edges regra→type de regra suprimida com classe
      `is-suppressed-edge`.
- [ ] T4. `nodes.tsx` + `flow.css`: badge/classe "suprimida" no `RuleNode`
      global; estilo esmaecido/tracejado do card e da edge.
- [ ] T5. `GameEventRulesCanvas.tsx`: `unconditionalTypes` ignora
      suprimidas; `NodeDetails` com botões Suprimir/Restaurar (chamando API +
      `reloadAll()`).
- [ ] T6. `GameEventRules.tsx` (tabela): ação suprimir/restaurar na linha
      global + indicador visual, paridade com o canvas.
- [ ] T7. Build verde (`tsc` + vite build).
- [ ] T8. Smoke Playwright: selecionar regra global ativa → "Suprimir para
      este jogo" → nó esmaece, edges tracejam, badge muda; selecionar de
      novo → "Restaurar" → volta ao normal; mesmo fluxo na visão em tabela;
      warning de redundância não cita event type de regra suprimida.
- [ ] T9. Commit + rebuild do admin-ui pro Vitor validar no :8089.
