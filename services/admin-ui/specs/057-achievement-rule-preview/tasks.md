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

## Follow-up 3: zebra sumida + warnings falsos de stat do host_world_dropin + polish de UX

Achados pelo Vitor testando a tabela de conquistas (2026-07-10):

1. **Zebra (`Table striped`) parou de alternar** depois do expander (T2 acima): a linha de
   prévia era SEMPRE renderizada no DOM (só o conteúdo colapsava via `Collapse`), dobrando o
   número de `<Table.Tr>` por conquista e fazendo toda linha de dado cair na mesma paridade
   par/ímpar. Corrigido: a segunda `<Table.Tr>` só existe no DOM quando `expanded` (mesmo padrão
   já usado em `GameSessions.tsx`).
2. **Warnings de "stat não declarado" falsos** no jogo Achievements Test, mesmo com o stat
   corretamente cadastrado no perfil do modo `host_world_dropin`. Causa: `statSuggestions`
   pulava esse arquétipo explicitamente ("não declara scoringStats") — comentário desatualizado
   desde que `scoringStats` foi adicionado a esse arquétipo (achievements/018). Removido o skip;
   os 66 achievements do jogo foram de N warnings pra 0 sem precisar cadastrar nada a mais —
   confirma que não havia gap de dado, só o bug de leitura no admin-ui.
3. **UX do overlay do canvas**: botão "Ver regra"/"Editar regra" migrou pra dentro do canvas
   (`Panel` top-right do `FlowCanvas`, mesmo padrão do canvas de Event Rules) em vez de uma
   `Group` abaixo ocupando espaço vertical; X explícito no mesmo canto pra fechar; cursor
   `pointer` explícito nos `Badge` de critério (a Mantine seta `cursor: default` no próprio
   `Badge`, sobrescrevendo o herdado do `Group` clicável ao redor).

## Follow-up 4: editar conquista deixa de ser modal — inline no expander, com prévia ao vivo

Pedido do Vitor: o modal de editar conquista escondia o contexto da tabela/canvas. Substituído
por edição INLINE no mesmo expander da prévia (T2): clicar no lápis expande a linha já em modo
edição — clicar na tag do critério continua abrindo só a prévia read-only, sem editar nada.

- **Layout**: o mini-canvas, que ocupava 100% da largura do expander, passa a dividir espaço
  com o formulário — 20% form (esquerda) / 80% canvas (direita, `Group wrap="nowrap"` com
  `Box` de `flex: "0 0 20%"` e `flex: "1 1 0%"`).
- **Prévia ao vivo**: em vez de passar o catálogo/foco "crus" pro `RuleFocusMiniCanvas`, quando
  a linha está em edição é montada uma cópia do catálogo com a conquista sendo editada
  substituída pelos valores AINDA NÃO salvos do form (`editForm.values`), e o `focusNodeId`
  segue o campo de critério ao vivo — trocar entre evento/stat, mudar a chave, ou mudar a meta
  atualiza o grafo (e o botão "Ver regra", que também passou a depender do critério ao vivo) na
  hora, sem precisar salvar. Verificado: mudar Meta de 1→999 atualiza o label do nó na hora;
  trocar de métrica pra evento (`match_won`) reancora o grafo pro event type e faz o botão "Ver
  regra" aparecer, tudo antes de qualquer submit.
- **Botões**: Salvar (PUT existente) · Resetar (volta aos valores do momento em que a edição
  abriu — `editSnapshot`, local, sem chamada ao servidor — diferente de `editForm.reset()`, que
  voltaria pros `initialValues` genéricos do form, não pro que estava salvo desta conquista) ·
  Cancelar (mesmo efeito do X que já existia no canvas — fecha e descarta sem salvar).
- **Consistência de estado**: `focusRow(id)` centraliza a troca de linha expandida — sempre que
  o alvo muda, `editing` é limpo se não bater com o novo `id`, evitando um estado "editando"
  órfão apontando pra uma conquista que não é mais a linha expandida (ex.: clicar na tag de
  critério de OUTRA linha enquanto uma está em edição).
- Modal de edição removida inteiramente (`Modal opened={!!editing}`); `id` continua não-editável
  (a identidade do nó no grafo nunca muda no meio de uma edição).
- Verificado com Playwright (build limpo + fluxo completo no jogo Achievements Test): sem modal
  no DOM, Salvar persiste e fecha, Cancelar descarta e fecha, Resetar reverte o form sem tocar o
  servidor, prévia comum (clique na tag) continua 100% de largura sem form.
