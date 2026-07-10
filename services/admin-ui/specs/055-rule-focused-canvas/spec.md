# Spec 055 — canvas focado por regra + lista separada

**Serviço:** admin-ui
**Relaciona:** specs 043–054 (editor visual de event rules), achievements/017 (V2b suppress)
**Depende de:** nada (o CRUD de regras já é granular por id — confirmado, seguro esconder nós no canvas). **Não muda nada no backend.**

## Contexto

O teste de estresse do motor de regras (66 achievements, 2 modos — ver memória `achievements-test-stress-demo`) validou o motor, mas expôs um problema de escala do **canvas**: com >20 achievements ele renderiza todos os nós de uma vez (`buildRulesGraph`, sem filtro/paginação), a coluna de achievements fica poluída (sem agrupamento visual, diferente das *sources*, que já têm seções por `SOURCE_GROUPS`) e fica difícil achar/depurar uma regra específica.

`GameEventRules.tsx` já tem uma visão de tabela (`view === "table"`) com as colunas certas (escopo, suprimida, arquétipo, match, event types) — mas hoje ela é uma **alternativa mutuamente exclusiva** ao canvas (`SegmentedControl`), não conectada a ele: trocar de view não leva contexto nenhum.

Decisão de design (usuário, 2026-07-10): o canvas passa a exibir **só os nós atrelados à regra selecionada** numa lista separada, com uma expansão opcional pra regras vizinhas que compartilham nós. Achado técnico que baixa o custo: `pathHighlight(edges, selectedId)` (`lib/rulesGraph.ts:357`) já calcula exatamente "nós alcançáveis a montante/jusante da regra selecionada" — hoje só pra destacar (dimming), a mesma travessia serve pra **filtrar** (esconder o resto).

## Comportamento (EARS)

1. **QUANDO** a aba de event rules é aberta, **O SISTEMA DEVE** exibir a lista de regras (reaproveitando as colunas já existentes na `view="table"` de hoje: escopo, suprimida, arquétipo, match, event types) e o canvas **lado a lado**, com o canvas **vazio** (estado "selecione uma regra") até uma regra ser escolhida.
2. **QUANDO** o usuário seleciona uma regra na lista, **O SISTEMA DEVE** popular o canvas só com: a própria regra, sua(s) source(s) direta(s), seu(s) event type(s), e o(s) achievement(s) alcançados a partir deles — o mesmo conjunto que `pathHighlight` já calcula — escondendo (não só destacando) qualquer nó fora desse conjunto.
3. **QUANDO** o usuário clica num nó do tipo `rule` diretamente no canvas (fluxo que já existe hoje via `selectedId`), **O SISTEMA DEVE** também sincronizar a seleção de volta pra lista (mesma regra fica destacada/selecionada nos dois lados) — os dois pontos de entrada (lista e canvas) convergem pro mesmo estado de foco.
4. **QUANDO** o usuário ativa o toggle opcional "mostrar regras relacionadas" (desligado por padrão), **O SISTEMA DEVE** adicionalmente exibir qualquer outra regra que compartilhe ao menos uma source ou event type com a regra focada (um salto a mais que o `pathHighlight` da regra base) — e as sources/event types/achievements que essas regras vizinhas trazem consigo.
5. **QUANDO** o usuário desmarca uma regra (deseleciona na lista, ou clica em "limpar seleção"), **O SISTEMA DEVE** voltar o canvas ao estado vazio do item 1.
6. **QUANDO** o usuário busca por texto (campo de busca já existente no canvas), **O SISTEMA DEVE** aplicar a busca só dentro do subgrafo atualmente focado (não busca mais no grafo inteiro do jogo) — busca deixa de ser mecanismo de navegação primária; a lista assume esse papel.
7. **O SISTEMA DEVE** manter os dois fluxos de criação de regra já existentes sem alteração: o ghost node in-canvas (specs 050/052) e o Modal da `view="table"` — nenhum dos dois está no escopo desta spec (ver "Fora de escopo").
8. **QUANDO** a lista de regras tem itens suficientes para exigir rolagem, **O SISTEMA DEVE** manter um campo de busca/filtro por texto na própria lista (independente da busca do canvas, item 6) — filtra as linhas visíveis da lista, não afeta o canvas até uma regra ser selecionada.

## Critérios de aceite

- Jogo com 66 achievements/regras (fixture do "Achievements Test"): abrir a aba renderiza a lista com todas as regras e o canvas vazio — nenhum node do grafo geral aparece antes de uma seleção.
- Selecionar uma regra na lista → canvas mostra só o subgrafo dela (contagem de nós = o que `pathHighlight` computaria manualmente para aquela regra).
- Clicar num node `rule` dentro do canvas (já focado numa regra A) → se for um node de uma regra diferente (trazido pelo toggle de relacionadas), o foco muda pra essa regra, a lista acompanha.
- Editar match/eventTypes de uma regra focada (form in-canvas já existente) → `PUT` pontual por id continua funcionando sem mudança de contrato; após salvar, o subgrafo focado se atualiza (a regra pode ganhar/perder event types).
- Toggle "regras relacionadas" ligado numa regra cujo event type é compartilhado por 3 outras regras → as 3 aparecem também, junto das sources/achievements delas.
- Deselecionar → canvas volta ao estado vazio; nenhum erro de nó "órfão" (edge sem node correspondente) no React Flow.
- Build/lint/typecheck do admin-ui verdes; sem mudança em nenhum client HTTP (`lib/api.ts`) — puramente client-side.

## Fora de escopo

- Unificar os dois fluxos de criação de regra (ghost in-canvas vs. Modal da tabela) — duplicação já existente, não piora nem melhora com esta spec.
- Mudar o mecanismo de busca do canvas além de restringir seu escopo ao subgrafo focado (item 6) — sem busca "global" que pule direto pra uma regra (poderia ser um follow-up).
- Qualquer mudança de backend/API — o CRUD de regras já é granular por id, confirmado seguro.
- Paginação/virtualização da lista de regras — se 66 linhas numa lista rolável já for suficiente, não precisa de mais nada agora; reavaliar só se um jogo real tiver centenas de regras.
