# Tasks 058 — canvas ocupando a aba Event Rules (chrome flutuante)

- [x] T1. `components/flow/FlowCanvas.tsx`: novo prop `dock?: ReactNode` → `<Panel position="top-left">`
- [x] T2. `lib/eventRules.ts`: mover `matchesListFilter` (hoje local em `GameEventRules.tsx`) pra cá, exportada
- [x] T3. `components/flow/nodes.tsx`: `RuleDraftData` ganha `archetypePicker?`; `RuleDraftNode` renderiza um `<Select>` compacto no topo do body quando `!editing && d.archetypePicker`
- [x] T4. `GameEventRulesCanvas.tsx`: remover o `<Select>` de arquétipo da toolbar; novo estado `dockCollapsed`/`listFilter`/`draftArchetype`; "Nova regra" e `saveDraft` passam a usar `draftArchetype` (só quando `archetypeOptions.length > 1`) em vez do arquétipo ativo fixo; `draftData` ganha `archetypePicker` condicional
- [x] T5. `GameEventRulesCanvas.tsx`: novo componente local `EventRulesDock` (lista compacta + toolbar + popover de info, tudo num painel colapsável só) — badges de event type coloridas por `ARCHETYPE_META[r.archetype].color`, sem texto de arquétipo separado; reaproveita `deleting`/`toggleSuppress`/`setDeleting` já existentes no canvas
- [x] T6. `GameEventRulesCanvas.tsx`: `FlowCanvas` recebe `dock={<EventRulesDock .../>}`; canvas passa a renderizar sempre (sem o ramo `!draft && !focusRuleId` que hoje esconde o `FlowCanvas` inteiro); `height` recalibrado pra ocupar a aba sem scroll de página
- [x] T7. `GameEventRules.tsx`: reduzir a um shell de dado — remove `Group`/`Box` de layout, texto explicativo fixo, modal de delete e `toggleSuppress`/`confirmDelete` próprios (duplicavam os do canvas); só fetch + sync de `pendingRuleFocus` + render do `GameEventRulesCanvas`
- [x] T8. `tsc --noEmit` + `npm run build` limpos
- [x] T9. Smoke manual (Playwright) no jogo "Achievements Test" (2 arquétipos): sem scroll de página; dock recolhe/expande preservando foco; info popover funciona; selecionar regra na lista do dock foca o canvas; "Nova regra" pede arquétipo (2 opções) e cria no arquétipo certo; badges de event type coloridas por arquétipo, sem texto de arquétipo na coluna Escopo; deletar/suprimir pela lista do dock funciona
- [x] T10. Smoke de regressão num jogo de 1 arquétipo só ("Packet Arena", só queue_arena): "Nova regra" NÃO pede arquétipo
- [x] T11. Smoke de regressão geral: regras relacionadas ("Stats" toggle), suprimir/restaurar regra global pela lista do dock, editar regra do jogo via hover no card, navegação vinda de Achievements ("Editar regra") — tudo funcionando idêntico ao comportamento pré-058
- [x] T12. Calibrar `height` do `FlowCanvas` via medição real (Playwright `getBoundingClientRect` do `.flow-wrap` + `scrollHeight` do documento) — `calc(100vh - 224px)`, zero scroll de página confirmado em 1700×1000

## Verificação real (jogo "Achievements Test", 2 arquétipos: host_world_dropin + queue_arena)

- `scrollHeight` do documento == `clientHeight` da viewport (1000 == 1000) — sem scroll de página.
- Dock: cabeçalho "Event Rules 13" (contagem), popover de info, botão atualizar, chevron de recolher — tudo funcionando; lista com 13 regras, scroll próprio quando excede a altura do painel.
- Badges de event type na lista: grape (host_world_dropin) / blue (queue_arena) — confirmado visualmente, sem texto de arquétipo na coluna Escopo (só badge GLOBAL/JOGO, agora sem truncar).
- "Nova regra" nesse jogo mostra o Select "Host World Drop-in"/"Queue Arena" no topo do form; criação end-to-end testada trocando pra Queue Arena, salvando, e confirmando via API que a regra nasceu com `archetype: "queue_arena"` (não o arquétipo ativo do canvas) — removida depois.
- Jogo "Packet Arena" (só queue_arena): "Nova regra" não mostra nenhum Select, só as duas primeiras condições direto.
- Regressão: editar via hover no card da regra, suprimir/restaurar regra global pela lista, toggle "Stats" de relacionadas, navegação "Editar regra" vinda da aba Achievements — todos passando sem erro de console/pageerror.

## Follow-up: calibração de `height` (spec item 1)

Estimativa inicial (`calc(100vh - 200px)`) deixava ~21px de sobra (scroll de página residual). Medido via `getBoundingClientRect()` do `.flow-wrap` + `document.documentElement.scrollHeight`: o offset real até o topo do canvas nessa página é ~205px, mais ~16px de padding/margin abaixo do canvas (contexto do `Tabs.Panel`/`Stack` do `GameDetail`). Ajustado pra `calc(100vh - 224px)` — zero scroll confirmado. Mesma disciplina de calibração por medição (não achismo) já usada nas alturas de `rulesGraph.ts`.

## Follow-up: rail de controles ao lado da lista + fit considerando a área reservada do dock

Dois ajustes pedidos pelo Vitor testando a v1 do dock (lista + toolbar empilhados num painel único, em cima da lista):

1. **Toolbar virou um rail vertical compacto** (`EventRulesToolbarRail`), flutuando ao LADO da lista (não mais empilhado acima dela) — três `ActionIcon`s (Nova regra / filtrar+buscar / regras relacionadas), cada um abrindo um `Popover` com o conteúdo que antes ocupava a barra fixa. `EventRulesDock` passou a ser só um `<Group>` horizontal com o rail + a lista, os dois dentro do mesmo `dock` (dá pra usar layout de botão/input diferente nesse contexto flutuante, sem amarrar no padrão de formulário de página comum).

2. **`fitView` passou a reservar o espaço real do dock**: antes, o autofit do canvas (`AutoFit` em `FlowCanvas.tsx`) considerava a área INTEIRA do canvas pra centralizar os nós — como o dock é um painel flutuante por CIMA do canvas (não reduz o espaço "de verdade"), nós focados podiam acabar posicionados por baixo da lista. Descoberta: o `padding` do `fitView` do React Flow aceita valores ASSIMÉTRICOS por lado (`{ left, right, top, bottom }`, não só um número uniforme) — é o mecanismo oficial pra reservar espaço de UI sobreposta. `FlowCanvas.tsx` agora mede a largura REAL do dock via `ResizeObserver` (não um número fixo torcido pra bater com o CSS) e passa isso como `padding.left` do fitView. Verificado numericamente: dock ocupa até x=628px, primeiro node do canvas some só a partir de x=637px — zero sobreposição.

Verificado visualmente e por medição (`getBoundingClientRect` de `.react-flow__panel` vs `.react-flow__node`): 0 nós sobrepondo o dock após focar uma regra; popovers do rail (filtro/busca e regras relacionadas) abrem e funcionam.

## Follow-up 2: rail movido pro lado direito da lista, orientação horizontal

Ajuste fino pedido pelo Vitor: o rail (item 1 do follow-up acima) nasceu vertical, à ESQUERDA da lista — pediu pra ficar à DIREITA e em fileira horizontal. `EventRulesToolbarRail` trocou `<Stack>` por `<Group wrap="nowrap">` (ícones lado a lado); `EventRulesDock` inverteu a ordem de render (lista primeiro, rail depois, dentro do mesmo `<Group>` horizontal). Os popovers dos ícones, que abriam pra direita (`position="right-start"`, fazia sentido numa coluna vertical), passaram a abrir pra baixo (`position="bottom-start"`) — numa fileira horizontal, abrir pra direita sobreporia o próximo ícone.

O `fitView` com padding reservado (follow-up 1) não precisou de nenhum ajuste adicional — como mede a largura real do dock inteiro via `ResizeObserver` (não um número fixo), absorveu a mudança de layout automaticamente. Reverificado: dock agora ocupa até x=692px (mais largo, rail horizontal ocupa mais espaço que vertical), primeiro node do canvas em x=701px — zero sobreposição, mesma margem de respiro de antes.

## Follow-up 3: reserva de espaço só da LISTA (não do rail) + "regras relacionadas" vira toggle inline

Dois pedidos do Vitor testando o rail horizontal:

1. **"Regras relacionadas" deixou de ser um `Popover`** (balãozinho flutuante) — virou um `ActionIcon` toggle simples (`relatedOpen`, estado local do rail): ligado, os chips Facts/Modo/Stats + o switch "qualquer limiar" aparecem INLINE na própria fileira, expandindo o rail pra direita (com um separador `borderLeft` antes deles), em vez de abrir uma bolha. Os outros dois ícones (Nova regra, filtrar/buscar) continuam como antes — só o de relacionadas tinha conteúdo 100% feito de toggles (chips + switch), então fazia sentido ser ele mesmo um toggle.

2. **A reserva do `fitView` media o dock INTEIRO (lista + rail)** — errado, porque o rail é curto (só a faixa do topo) e nunca tampa node nenhum; reservar a largura dele também só empurrava o grafo mais longe do que precisava. Resolvido restruturando `dockReservedWidth`: em vez do `FlowCanvas` medir sozinho (via `ResizeObserver` no wrapper do `dock` inteiro), agora é o CHAMADOR (`GameEventRulesCanvas.tsx`) que mede — só o painel da LISTA (`listRef` dentro de `EventRulesDock`, repassado por `onListPanelWidthChange`) — e passa o número explícito pro `FlowCanvas` via `dockReservedWidth`. `FlowCanvas` ficou mais simples (não mede mais nada sozinho, só recebe o número).

Verificado com medição separada de cada painel (`getBoundingClientRect` da lista vs. do rail, não da caixa combinada — o teste inicial com a caixa combinada dava falso positivo de sobreposição): 0 nós sobrepondo a lista E 0 nós sobrepondo o rail, tanto com o toggle de relacionadas fechado quanto aberto (rail crescendo de ~102px pra ~315px de largura) — como o rail é curto (38px de altura, só a fileira do topo), mesmo crescendo bastante pra direita nunca chega a tampar node nenhum na prática.

## Follow-up 4: margem maior entre dock e nós + filtro no header da lista

Dois ajustes finos pedidos pelo Vitor:

1. **`FIT_MARGIN` foi de 24px pra 90px** — o respiro mínimo teórico (garantido pelo `padding` assimétrico do fitView) ficou visualmente apertado depois da calibração do follow-up 3 (a reserva agora é exata, só da lista, sem sobra). Ajustado por iteração visual (medir `listRect.right` vs. `firstNodeLeft` via Playwright, comparar com o screenshot) até ficar confortável.

2. **Filtro de lista migrou pro header do painel da lista** — antes vivia escondido dentro do popover "filtrar/buscar" do rail, junto com a busca no subgrafo. Removido o texto "Event Rules" e o header agora é `[TextInput "filtrar…"] [Badge dinâmico] [info] ... [atualizar] [recolher]` — o Badge deixou de mostrar `rules.length` (total fixo) e passou a mostrar `filtered.length` (dinâmico, reflete o filtro ativo). O popover do rail ficou só com "Buscar no subgrafo" (a busca que destaca nodes no canvas, conceito diferente do filtro da lista). Verificado: filtrar "boss" no jogo de teste reduz a contagem de 13 pra 4 em tempo real.
