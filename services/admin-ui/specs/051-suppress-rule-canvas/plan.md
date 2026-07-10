# Plan 051 — Suprimir/restaurar regra global no canvas (V2b)

## Decisões técnicas

1. **`EventRule.suppressed`** (lib/api.ts) — campo novo, sempre presente
   (backend já devolve `false` pra regras de jogo). `api.suppressEventRule`/
   `api.unsuppressEventRule` seguem o mesmo padrão de `deleteEventRule`
   (`POST`/`DELETE` sem body, `Promise<void>`).
2. **Nó (`RuleNode`, `components/flow/nodes.tsx`)**: o badge hoje é estático
   pra globais (`{global ? "read-only" : ...}`). Vira condicional em 3 vias —
   `!global` (fluxo de edição existente, inalterado) / `global &&
   !suppressed` ("read-only") / `global && suppressed` ("suprimida", cor de
   alerta, ícone `IconBan`). Classe do card ganha `flow-card--suppressed`
   quando suprimida, aplicada **junto** de `flow-card--rule-global` (não
   substitui — a supressão é um estado, não outro tipo de regra).
3. **CSS (`flow.css`)**: `.flow-card--suppressed` — `opacity: 0.45`, borda
   tracejada (mesma cor de alerta do badge nas bordas do card, não um
   amarelo genérico — reusa a paleta já usada pro warning de redundância).
   `.react-flow__edge.is-suppressed-edge` — mesma receita de
   `.is-draft-edge` (tracejada), sem a animação (a edge suprimida não é um
   rascunho "vivo", é um caminho morto).
4. **`rulesGraph.ts`**: as edges regra→type já são construídas em loop sobre
   `typeSources` (linha ~317, `push(ruleMetas[ri].id, typeId(et))`) — ganham
   uma checagem `rules[...].suppressed` pra escolher a classe (`cls`)
   passada ao `push` (hoje só `undefined`, exceto ghosts). Não precisa de
   estrutura de dado nova: `EventRule.suppressed` já está no objeto `r` que o
   loop já percorre.
5. **`unconditionalTypes` (`GameEventRulesCanvas.tsx`, linha ~173)**: filtro
   ganha `&& !r.suppressed` — uma regra suprimida não emite nada, não pode
   tornar outra "redundante". Mesmo ajuste não é necessário no cálculo de
   `redundant` dentro de `NodeDetails` (linha ~607) porque ele já lê de
   `unconditionalTypes`, que passa a vir correto.
6. **`NodeDetails` (mesma página, bloco `node.type === "rule"`)**: hoje
   bifurca só em `global ? <texto> : <botão remover>`. Vira 3 vias: `!global`
   (inalterado) / `global && !suppressed` (texto atual + botão "Suprimir
   para este jogo", `onClick` chama `api.suppressEventRule` e
   `reloadAll()`) / `global && suppressed` (texto novo + botão "Restaurar",
   chama `api.unsuppressEventRule` e `reloadAll()`). Sem modal — mesmo
   raciocínio de reversibilidade de um clique já usado noutros lugares do
   console (ex.: toggles de settings).
7. **Tabela (`GameEventRules.tsx`)**: a célula de ações (hoje só `{r.scope ===
   "game" && <ActionIcon remover>}`) ganha o par pro caso `scope ===
   "global"`: `ActionIcon` de suprimir/restaurar (ícone `IconBan`/
   `IconRotate2` ou equivalente), mesmas chamadas de API + `reload()`
   (a tabela só recarrega `rules`, não o catálogo — `reload`, não
   `reloadAll`, mesmo padrão do delete existente ali). Badge de scope na
   primeira coluna ganha um indicador extra (`opacity` na row ou badge
   "suprimida" adicional) quando `r.suppressed`.

## Arquivos

- `src/lib/api.ts` — `EventRule.suppressed`; `suppressEventRule`,
  `unsuppressEventRule`.
- `src/lib/rulesGraph.ts` — classe `is-suppressed-edge` nas edges regra→type
  de regra suprimida.
- `src/components/flow/nodes.tsx` — `RuleNode`: badge/classe condicionais a
  `rule.suppressed`.
- `src/components/flow/flow.css` — `.flow-card--suppressed`,
  `.is-suppressed-edge`.
- `src/pages/GameEventRulesCanvas.tsx` — `unconditionalTypes` filtra
  suprimidas; `NodeDetails` com os 2 botões novos.
- `src/pages/GameEventRules.tsx` — ação suprimir/restaurar na tabela +
  indicador visual na linha.

## Riscos / atenção

- Depende de `achievements-service/017` estar mergeado e os containers
  rebuildados antes de testar ponta a ponta (endpoints `.../suppress`
  precisam existir no BFF).
- Reconferir que a paleta de cor do badge/borda "suprimida" não colide
  visualmente com a de "drift" (vermelho, já usada nos cards de fonte) nem
  com a de redundância (amarelo/`yellow.5`) — usar uma cor distinta
  (ex.: laranja) evita ambiguidade entre os três tipos de alerta do canvas.
