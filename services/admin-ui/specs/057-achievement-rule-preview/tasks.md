# Tasks 057 — prévia de regras/fonte no catálogo de achievements

- [x] T1. `GameAchievements.tsx`: componente local `RuleFocusMiniCanvas` (`buildRulesGraph` sem filtro de arquétipo, `pathHighlight`+`repackVisible`, `MINI_NODE_TYPES` sem draft/ghost, sem overlay, nodes forçados `selectable: false, draggable: false`)
- [x] T2. `GameAchievements.tsx`: `expandedCriteriaId` state; clique na tag de critério expande (`Collapse`, mesmo padrão de `GameSessions.tsx`); `Table.Td colSpan` com a prévia + botão de navegação (achievement por evento) ou só prévia (achievement por métrica)
- [x] T3. `GameAchievements.tsx`: `targetRuleFor(a)` (prioriza `scope=game`, senão primeiro `global`) + `onNavigateToRule` prop
- [x] T4. `GameDetail.tsx`: `Tabs` controlado (`activeTab`/`setActiveTab`); `pendingRuleFocus` state; passa callbacks pra `GameAchievements`/`GameEventRules`
- [x] T5. `GameEventRules.tsx`: props `pendingRuleFocus`/`onConsumePendingRuleFocus`; `useEffect` seta `focusRuleId` e consome; repassa `autoEditRuleId` pro canvas
- [x] T6. `GameEventRulesCanvas.tsx`: prop `autoEditRuleId`; `useEffect` chama `startEdit(rule)` quando bate com uma regra do jogo carregada
- [x] T7. `tsc --noEmit` + `npm run build` limpos
- [x] T8. Smoke manual (Playwright) no jogo "Achievements Test": prévia por evento, prévia por métrica (sem botão), navegação "Editar regra" (jogo) e "Ver regra" (global), sem interação nos nodes da prévia

## Follow-up: crash em "Editar regra" (mini-canvas por MÉTRICA vazio + tela em branco na navegação pro jogo)

Dois bugs reais achados testando ao vivo com Playwright (`page.on("pageerror"/"console")` —
sem esses listeners os dois passariam batidos como timeout genérico):

1. **Prévia de achievement por métrica vinha totalmente vazia** (nem o próprio nó do
   achievement aparecia). Causa: `buildRulesGraph` só constrói os nós/edges de achievement
   por métrica (`metricAchs`) quando recebe `opts.stats` — `RuleFocusMiniCanvas` chamava sem
   esse parâmetro. Corrigido computando um `statsInput: StatInfo[]` sintético a partir de
   `catalog` (todas as `statKey` distintas, `declared: true` incondicional — drift não
   importa nesta micro-prévia) e passando como `opts.stats`.

2. **Clicar "Editar regra" (regra do JOGO) quebrava a página inteira** (`TypeError: Cannot
   read properties of undefined (reading 'type')`, tela em branco). Causa raiz: no mount
   vindo de Achievements, `autoEditRuleId` já chega preenchido (deriva direto da prop
   `pendingRuleFocus`), mas `focusRuleId` ainda é `null` nesse primeiro render (é estado
   próprio do `GameEventRules`, que só é atualizado por um efeito assíncrono depois). O
   efeito de sincronização de arquétipo só rodava quando `focusRuleId` era truthy — ficava
   pulado — enquanto `startEdit` (disparado por `autoEditRuleId`) já montava o rascunho de
   edição pra uma regra fora do arquétipo ainda ativo. `archRules` (filtrado pelo arquétipo
   errado) não continha a regra, e `buildRulesGraph` quebrava ao processar
   `draft.editingRuleID` apontando pra uma regra ausente. Corrigido em duas frentes
   (`GameEventRulesCanvas.tsx`): o efeito de sincronização agora também considera
   `autoEditRuleId` (`focusRuleId ?? autoEditRuleId`), e `archRules` passou a incluir
   defensivamente a regra em edição mesmo se `arch` ainda não tiver convergido
   (`r.archetype === arch || r.id === draft?.editingRuleID`) — cobre esse e qualquer race
   futura equivalente.

Reverificado após as correções: prévia por métrica mostra os 2 nós corretos (source STAT →
achievement), "Ver regra" (global) e "Editar regra" (jogo) navegam e abrem o form/overlay
esperado, sem erros de console/pageerror.
