# Spec 056 — criação de regra dinâmica no canvas + foco também na edição

**Serviço:** admin-ui
**Status:** Done
**Relaciona:** spec 055 (canvas focado por regra + lista separada)
**Depende de:** nada (client-side; `saveDraft` já manda `eventTypes` como array plano, sem distinguir "linkado" de "novo" — confirmado, sem risco de duplicar).

## Contexto

Depois da spec 055, criar uma regra nova (ghost in-canvas) ainda mostra o **grafo inteiro** do arquétipo — `draft !== null` desliga o filtro de foco por completo (decisão original: "não mexer no fluxo de criação"). Na prática (Vitor testando), isso é ruim: a mesma poluição visual que a 055 resolveu pra visualização volta na hora de criar.

Achado ao investigar: `saveDraft` (`GameEventRulesCanvas.tsx:304-342`) já concatena `[...draft.linked, ...draft.news.map(n => n.value)]` num array plano — não existe diferença de dado entre "linkar um tipo existente" e "criar um tipo novo com o mesmo nome". A única razão de existirem dois mecanismos hoje é de **interação**: linkar um tipo existente é clicar no nó dele; criar um novo é digitar na caixinha ghost (que já tem autocomplete com os tipos existentes, `typeSuggestions`). Decisão do Vitor: resolver digitando o nome no autocomplete do ghost já é suficiente — não precisa do nó do tipo aparecer.

Achado colateral: **editar** uma regra existente (`draft.editingRuleID` setado) também desliga o filtro hoje, pelo mesmo `if (!draft) {...}` — ou seja, editar também mostra o grafo inteiro, quando já teria um foco natural e óbvio (a própria regra em edição). É a mesma classe de problema, resolvida de graça reaproveitando o `focusPath` que a 055 já construiu.

## Comportamento (EARS)

1. **QUANDO** o usuário clica em "Nova regra" (criação, `draft.editingRuleID === null`), **O SISTEMA DEVE** exibir o canvas só com a caixinha da regra em rascunho e a caixinha ghost de event type — nenhuma source, event type existente ou achievement visível ainda.
2. **QUANDO** o usuário digita/seleciona uma chave válida numa linha de match (autocomplete já existente, `keySuggestions`), **O SISTEMA DEVE** fazer aparecer no canvas o nó da source correspondente (eco visual) — sem exigir clique pra "descobrir" a fonte.
3. **QUANDO** o usuário apaga ou invalida uma chave de uma linha, **O SISTEMA DEVE** esconder de volta a source correspondente (se nenhuma outra linha ainda referenciar a mesma chave).
4. **O SISTEMA NÃO PRECISA** exibir nós de event type existentes durante a criação — linkar um tipo já existente continua funcionando ao digitar o nome exato no autocomplete do ghost (`typeSuggestions`, já implementado); `saveDraft` já trata isso corretamente (sem duplicar).
5. **QUANDO** o usuário clica em "editar" numa regra (`draft.editingRuleID` setado), **O SISTEMA DEVE** continuar mostrando o subgrafo focado da regra (mesmo `focusPath`/`repackVisible` da spec 055) em vez do grafo inteiro — a regra em edição vira o nó `ruleDraft` na mesma posição, como já acontece hoje.
6. **QUANDO** a regra em edição é uma "vizinha" trazida pelo toggle "regras relacionadas" (não a `focusRuleId` original), **O SISTEMA DEVE** focar nela (não na regra originalmente selecionada na lista) — o foco durante edição segue `draft.editingRuleID`, não a seleção externa.
7. **Cancelar ou salvar o rascunho** (criação ou edição) **DEVE** voltar o canvas ao estado de foco anterior (a regra que estava selecionada na lista antes de abrir o form), sem comportamento novo além do que já existe.

## Critérios de aceite

- Clicar "Nova regra" com o jogo "Achievements Test" (13 regras, várias sources/event types) aberto → canvas mostra só 2 caixinhas (regra + tipo), nada mais.
- Digitar `event` numa linha de match → o node da source `event` aparece; apagar o valor → some de novo.
- Digitar um event type já existente (ex.: `WORLD_JOINED`) na caixinha ghost e salvar → a regra criada tem esse type no `eventTypes` (mesmo request de sempre, sem mudança de payload).
- Editar uma regra global suprimida/regra do jogo qualquer → canvas mostra só o subgrafo dela (sources+tipos+achievements dela), não o grafo inteiro.
- Editar uma regra vizinha (aparecida via "regras relacionadas") → o foco muda pra ela durante a edição; cancelar volta pro foco original.
- `tsc --noEmit` + `npm run build` limpos; smoke manual via Playwright confirma os itens acima sem erro de console.

## Fora de escopo

- Qualquer mudança no backend/API — comportamento 100% client-side.
- Mudar a UI do ghost de event type (autocomplete já existe, `typeSuggestions`) — só deixamos de exigir visualização do nó existente.
- Remover o mecanismo de clique-pra-linkar em `eventType` — continua existindo e é usado durante a edição (regra focada pode ter tipos existentes visíveis).
