# Spec: 087-analytics-mode-scope-switch

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-14
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 086. Decisão de design discutida em texto com o Vitor antes
de implementar (única exceção parcial nesta fase: ele pediu minha opinião
sobre a proposta original antes de eu escrever qualquer código — ver
"Contexto").

## Contexto

O seletor de modo do Analytics (`GameAnalytics.tsx`) era um único `Select`
com N+1 itens: "Todos os modos" + um item por modo real. Isso misturava
duas coisas diferentes numa lista só — "Todos os modos" não é "mais um
modo", é o estado padrão/mais usado, perdido no meio da lista.

O Vitor propôs um `SegmentedControl` (Visão geral / Por modo) substituindo
o Select, com um Select de modo aparecendo TAMBÉM dentro da própria "Visão
geral". Dei minha opinião antes de implementar: a 2ª parte duplicava a
1ª — se "Visão geral" também filtra por um modo específico, ela vira
funcionalmente igual a "Por modo", e o switch para de separar algo de
verdade. Sugeri: `SegmentedControl` com 2 estados puros — "Todos os
modos" (sem seletor extra, tudo agregado) e "Por modo" (revela o Select
de modo, que só faz sentido existir ali). O Vitor aceitou a sugestão.

## Decisões

### Dois estados (`scope`), não um Select com sentinela

`selectedMode` (state antigo) virou dois states: `scope: "all" |
"by-mode"` (o `SegmentedControl`) e `selectedMode: string | null` (o
modeId real, só relevante quando `scope === "by-mode"`). Isso elimina o
sentinela `ALL_MODES` (`"__all__"`) que existia só pra contornar o Select
do Mantine não lidar bem com `value=""` — não é mais necessário porque o
Select de modo agora só é montado (e só recebe valor) quando `scope ===
"by-mode"`, nunca precisa representar "nenhum/todos" como opção da
própria lista.

### Auto-seleção ao entrar em "Por modo"

Ao clicar em "Por modo" pela 1ª vez (sem seleção prévia), o 1º modo da
lista é escolhido automaticamente (`modes[0]?.modeId`) — evita um estado
intermediário "escolha um modo" sem dado nenhum na tela; o usuário já
entra vendo alguma coisa e troca se quiser outro modo.

### Filtro de arquétipo inalterado

`selectedArchetype` (usado pra restringir o catálogo de widgets — spec
064) passou a depender de `scope === "by-mode"` em vez de `selectedMode
!== ALL_MODES`, mas a lógica de filtro em si (por arquétipo do modo
escolhido) não mudou.

## Critérios de Aceite

1. `tsc --noEmit` limpo.
2. `SegmentedControl` só aparece quando o jogo tem mais de 1 modo (mesma
   condição de antes, `modes.length > 1`).
3. Em "Todos os modos": nenhum Select extra, todos os arquétipos com dado
   aparecem, agregados — validado ao vivo (Neon Breach, 3 modos).
4. Em "Por modo": aparece o Select de modo ao lado do switch, já com o
   1º modo pré-selecionado; a tela filtra pra só a seção do arquétipo
   desse modo — validado ao vivo (trocar de "Raids Co-op" pra "Arena
   Ranqueada" trocou a seção de "Co-op Mission" pra "Queue Arena", com
   os números filtrados só daquele modo).
5. Voltar pra "Todos os modos" some com o Select de modo e volta ao
   agregado — validado ao vivo.
6. Sem erro de console em nenhuma transição.

## Fora de escopo

- Persistir a escolha de `scope`/modo entre visitas (localStorage) — não
  foi pedido; hoje reseta pra "Todos os modos" a cada entrada na aba,
  mesmo comportamento de antes (o Select também resetava).
