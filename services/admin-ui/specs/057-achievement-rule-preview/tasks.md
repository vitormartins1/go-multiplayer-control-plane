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

## Follow-up: prévia por métrica não agrupava os achievements irmãos do mesmo stat

Achado do Vitor testando ao vivo: pra evento, expandir qualquer achievement de um event type
mostra TODOS os achievements que escutam aquele type (visão em grupo, útil). Pra métrica, cada
achievement mostrava só ele mesmo + a fonte — mesmo quando 5 achievements do catálogo
compartilhavam o mesmo `statKey` (ex.: `animals_saved` com 5 tiers).

Causa: `pathHighlight` faz dois caminhos UNIDIRECIONAIS independentes a partir do nó focado (um
"upstream", outro "downstream") — não é uma busca bidirecional real que pivota em nós
intermediários. Pro lado de evento, o foco já era o nó do EVENT TYPE (o ponto de ramificação,
com uma edge de saída pra cada achievement que o escuta), então o caminho "downstream" pegava
todos os irmãos de graça. Pro lado de métrica, o foco era o nó do ACHIEVEMENT individual (uma
folha, sem saída) — nenhum dos dois caminhos unidirecionais a partir de uma folha alcança outra
folha do mesmo "galho".

Corrigido (`GameAchievements.tsx`): o foco da prévia por métrica passou do nó do achievement
(`achId(a.id)`) pro nó da FONTE do stat (`srcId(STAT_PREFIX + a.statKey)`) — o mesmo padrão já
usado pro lado de evento (focar no nó de ramificação, não na folha). Verificado com dado real:
stat só-métrica (`animals_saved`, 5 achievements) mostra os 6 nós certos (fonte + 5 irmãos).

### Follow-up 2: fonte de stat também trazia regras que só coincidem na chave (sem alimentar o achievement)

Testando o caso `deaths` (também usado como condição `stat.deaths ≤ 0` na regra
`flawless_invasion`), a prévia trazia 9 nós: os 5 achievements de métrica certos, MAS também a
regra, o event type dela e o achievement por evento dela — feedback do Vitor: essa regra "não
tem a ver com esse achievement específico", ele não queria essa relação aparecendo aqui (é
diferente do caso de evento, onde a regra realmente EMITE o event type focado — aqui a regra só
usa o mesmo stat como um dos vários filtros, coincidência de chave, não uma relação de
alimentação).

Causa: com o foco na fonte, `pathHighlight` seguia TODAS as edges que saem dela — tanto as de
MÉTRICA (fonte→achievement, a relação que importa aqui) quanto as de CONDIÇÃO DE REGRA
(fonte→regra, usada só pro lado de Event Rules). Corrigido: quando o nó focado é uma fonte de
kind `"stat"`, as edges candidatas ao `pathHighlight` são filtradas pra excluir qualquer edge
saindo da fonte que NÃO seja `className === "is-metric-edge"` — mantém só fonte→achievement
direto. Lado de evento inalterado (o foco continua no event type, sem essa restrição).
Reverificado: `deaths` volta a mostrar só 6 nós (fonte + 5 achievements); `animals_saved`
continua igual; o subgrafo por evento (`COOP_BOSS_HELPED`, com fact/regra/type/5 achievements)
não foi afetado.
