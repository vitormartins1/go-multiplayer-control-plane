# Spec 058 — canvas ocupando a aba Event Rules (chrome flutuante)

**Serviço:** admin-ui
**Status:** Done
**Relaciona:** spec 055 (canvas focado por regra), spec 056 (criação dinâmica), o painel recolhível de detalhe do node (2026-07-10)
**Depende de:** nada novo no dado — reorganização de LAYOUT sobre `GameEventRules.tsx`/`GameEventRulesCanvas.tsx`; reaproveita `ARCHETYPE_META` (`lib/archetypeMeta.ts`) e o padrão de painel flutuante recolhível já usado no `NodeDetails`.

## Contexto

Hoje a aba Event Rules é uma página com scroll: texto explicativo fixo no topo, depois uma `Group` lado a lado com uma coluna de lista (380px, com sua própria `ScrollArea`) e o canvas (com sua própria toolbar de controles acima dele). O canvas em si — a parte que realmente importa — fica espremido pela soma de todo esse chrome fixo.

Ideia do Vitor: já existe o padrão certo pra isso — o painel flutuante recolhível que a gente construiu pro detalhe do node (`NodeDetails`, `overlay` do `FlowCanvas`). É o mesmo mecanismo que deveria valer pra lista de regras e pra toolbar: viram painéis flutuando SOBRE o canvas, recolhíveis, em vez de chrome fixo consumindo espaço permanente. O canvas passa a ocupar toda a área da aba, sem scroll de página.

Junto, dois ajustes que ficaram mais óbvios de fazer nessa reorganização:
- **Seletor de arquétipo** (dropdown na toolbar): perdeu utilidade desde que o canvas passou a ser focado por regra (spec 055) — o arquétipo ativo já é sincronizado automaticamente a partir da regra selecionada. Só sobra útil no momento de criar uma regra nova sem nada focado ainda, em jogo com mais de um arquétipo — isso migra pro próprio formulário de criação.
- **Cor da badge de event type na listagem**: hoje é sempre violeta, com o nome do arquétipo escrito embaixo da badge de escopo. Passa a usar a cor do arquétipo (`ARCHETYPE_META[r.archetype].color` — mesma paleta já usada no header do `GameDetail`), o que deixa a linha mais compacta (uma informação a menos pra escrever por extenso) sem perder a informação.

## Comportamento (EARS)

1. **QUANDO** a aba Event Rules é aberta, **O SISTEMA DEVE** ocupar toda a altura disponível da viewport com o canvas, sem produzir scroll de página (nem vertical nem horizontal) em resoluções desktop padrão.
2. **O SISTEMA DEVE** manter a lista de regras como uma listagem real (mesmas colunas essenciais — escopo, event types, ação de remover/suprimir), reorganizada como um painel lateral flutuante sobre o canvas, recolhível/expansível (mesmo padrão do painel de detalhe do node).
3. **QUANDO** o painel da lista está recolhido, **O SISTEMA DEVE** mostrar um indicador compacto pra reabri-lo, preservando a regra selecionada e o subgrafo focado no canvas por baixo.
4. **O SISTEMA DEVE** mover os controles hoje na toolbar fixa (nova regra, regras relacionadas por kind, "qualquer limiar", busca no subgrafo) pra um painel compacto flutuante sobre o canvas — sem barra fixa ocupando espaço permanente na página.
5. **O SISTEMA DEVE** remover o seletor manual de arquétipo — o arquétipo ativo do canvas continua derivado automaticamente da regra focada (mecanismo já existente, spec 055).
6. **QUANDO** o usuário clica "Nova regra" e o jogo tem mais de um arquétipo cadastrado (`archetypeOptions.length > 1`), **O SISTEMA DEVE** pedir o arquétipo da regra dentro do próprio formulário de criação in-canvas — jogos com um único arquétipo não veem esse campo (valor implícito).
7. **O SISTEMA DEVE** substituir o texto explicativo fixo (parágrafo sobre como as regras funcionam) por um ícone/expander pequeno que mostra o mesmo conteúdo sob demanda, sem ocupar espaço permanente.
8. **NA** listagem de regras, **O SISTEMA DEVE** colorir a badge de cada event type com a cor do arquétipo da regra (`ARCHETYPE_META[r.archetype].color`) e **NÃO DEVE** mais exibir o nome do arquétipo como texto separado na coluna Escopo (a cor já comunica essa informação, com a mesma paleta já usada no header do `GameDetail`).
9. **O SISTEMA NÃO DEVE** alterar nenhum comportamento funcional já existente (foco por regra, edição/criação in-canvas, regras relacionadas, drift, supressão de regra global, quick-create de achievement, navegação vinda da aba Achievements) — é uma reorganização de layout/chrome, não de lógica.

## Critérios de aceite

- Jogo "Achievements Test" (2 arquétipos), viewport 1700×1000: abrir a aba Event Rules não produz scroll de página; o canvas ocupa a área inteira abaixo das tabs.
- Painel da lista: recolher e expandir preserva a regra focada e o subgrafo visível no canvas.
- Painel da toolbar: os controles (nova regra, relacionadas, busca) continuam funcionando idênticos ao comportamento atual, só reposicionados.
- Clicar "Nova regra" no jogo de teste (2 arquétipos) pede o arquétipo no formulário; num jogo com 1 arquétipo só, não pede.
- Badges de event type na listagem usam a cor do arquétipo correspondente (grape pra host_world_dropin, blue pra queue_arena, etc. — mesma paleta do `ARCHETYPE_META`); a coluna Escopo não mostra mais o nome do arquétipo como texto.
- Fluxos existentes (focar, editar, criar, relacionadas, drift, supressão, quick-create, navegação vinda de Achievements) continuam passando nos mesmos smoke tests já usados nas specs 055–057.
- `tsc --noEmit` + `npm run build` limpos.

## Fora de escopo

- Redesenho visual dos cards do grafo em si (cores/formas dos nodes, `flow-card--*` em `flow.css`) — só o chrome ao redor (toolbar/lista/texto) muda de lugar.
- Mudança de dado/API — 100% reorganização client-side.
- Persistir o estado recolhido/expandido dos painéis entre sessões (localStorage) — cada carregamento da aba parte do estado padrão definido no plan.
- Layout responsivo/mobile — a aba continua desktop-first, como o resto do admin-ui.
