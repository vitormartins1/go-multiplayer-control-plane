# Spec: 086-analytics-donut-layout

**Serviço:** admin-ui
**Status:** Implemented
**Data:** 2026-07-14
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 085 (iterada ao vivo com o Vitor olhando screenshots reais,
duas rodadas de ajuste no mesmo pedido).

## Contexto

Nos 4 widgets de Analytics que combinam um grid de `StatCard` com um
`DonutChart` (Resumo/queue_arena, Distribuição de colocação/battle_royale,
Desfecho das missões/coop_mission, Saúde das sessões/cross-arquétipo), o
donut ficava EMPILHADO abaixo do grid de stats — o card ficava bem mais
alto que o card "por dia" pareado ao lado (só um `LineChart` de 170px),
com bastante espaço vazio structural entre os dois.

## Decisões

### 1. Donut ao lado do grid de stats, não embaixo

Os 4 `render()` mudaram de `Stack` (vertical) para `Group align="center"
wrap="wrap"` (horizontal, com wrap pra mobile): `SimpleGrid` de stats com
`flex: "1 1 260px"` ao lado do `DonutChart`. `wrap="wrap"` é o que garante
a responsividade — em telas estreitas (`Grid.Col span base=12`) o Group
já teria só uma coluna de largura, então o donut cai pra baixo do grid
naturalmente, sem precisar de breakpoint dedicado.

### 2. `StatCard` ganhou `size="lg"` (opt-in)

Achado na 1ª rodada: um card raso (StatCard padrão, 1 linha) ao lado de um
donut de 140px ficava desproporcional — texto pequeno demais, muito
espaço vazio vertical. `StatCard` ganhou um prop `size?: "sm" | "lg"`
(default `"sm"`, preserva todo uso existente — Infrastructure, header do
jogo). Só os 4 widgets de Analytics passam `size="lg"` (padding maior,
valor em fonte maior).

**Achado/bug pego nessa rodada**: a 1ª tentativa também tinha posto
`h="100%"` no `Card` do StatCard, pra tentar igualar a altura dos cards
dentro do próprio `SimpleGrid`. Isso quebrou o wrap do label em grids de
1 linha (3 itens) — o texto ("Encerrada manualmente", "Concluída")
aparecia cortado no meio da palavra, sem reticências. Causa: `height:
100%` dentro de uma grid-track "auto" cria um loop de dependência de
tamanho no CSS Grid: o card colapsa pra uma altura menor que o texto
precisa, e o `Card` do Mantine tem `overflow: hidden` (pro
`border-radius` recortar o conteúdo), então o excesso de texto some sem
aviso. Removido — sem `h="100%"` o Card volta a crescer pelo conteúdo
normalmente.

### 3. Par 60/40 em vez de 50/50, cards do par com a MESMA altura

2ª rodada (pedido do Vitor revendo o resultado): mesmo com o donut ao
lado, valores grandes (`"4min 50s"`, `"10min 51s"`) ainda ficavam
apertados num card de 50% da linha. Os 4 pares (`summary`/`queue-daily`,
`placement-distribution`/`placement-daily`, `mission-distribution`/
`mission-daily`, `session-health`/`session-closed-daily`) mudaram de
`span: 6/6` pra `span: 8/4` — o lado com stats+donut ganha mais respiro
horizontal, o lado do LineChart (que não tem texto pra caber) sobra bem
com menos.

Junto, o `Grid` da seção deixou de forçar `align="flex-start"` (herdado
da spec 068, quando o donut empilhado embaixo do grid deixava o card
desproporcionalmente alto — motivo que não existe mais) — o comportamento
default do `Grid` é `align="stretch"`, então agora os dois cards de cada
par (com `Paper h="100%"`, novo) nascem com a MESMA altura, pedido
explícito do Vitor ("cada subseção... ficar alinhada com a mesma
altura"). `DonutChart` também encolheu de `size={140}` pra `size={120}`
(mais respiro pro grid de stats dentro do agora 8/12 = 67% da linha).

## Critérios de Aceite

1. `tsc --noEmit` limpo.
2. Nos 4 widgets, o donut aparece ao lado do grid de stats, não
   empilhado — confirmado via screenshot (Capoeira Souls/Queue Arena,
   Neon Breach/Battle Royale+Co-op Mission, Packet Arena).
3. Nenhum label de `StatCard` corta no meio da palavra — confirmado via
   screenshot em zoom da seção "Saúde das sessões" (o caso mais estreito,
   3 stats + label longo "Encerrada manualmente").
4. Cada par (stats+donut / gráfico "por dia") tem a mesma altura de card
   — confirmado via screenshot nos 3 jogos de referência.
5. Mobile (390px): `Group wrap="wrap"` empilha o donut abaixo do grid de
   stats sem quebrar layout — confirmado via screenshot.
6. Widget sem par (ex.: "Desfecho das missões" da Packet Arena, sem
   `missionDaily`) ocupa `span=8` sozinho na linha, sem esticar — mesmo
   comportamento de antes (só o número mudou de 6 pra 8).

## Fora de escopo

- O seletor de modo (list-select → switch Visão geral/Por modo) — pedido
  separado do Vitor na mesma conversa, tratado à parte (opinião dada,
  implementação não iniciada nesta spec).
- Padronizar a largura 8/4 nos widgets `span=12` (participations-role,
  raw-stats, participation-daily) — esses não têm par lateral, full-width
  continua sendo o layout certo pra eles.
