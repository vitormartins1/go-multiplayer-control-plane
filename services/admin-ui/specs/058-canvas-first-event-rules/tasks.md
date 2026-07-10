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
