# Spec: 068-analytics-layout-density-fix

**Serviço:** admin-ui
**Status:** Done
**Data:** 2026-07-13
**Parte de:** segunda iteração de feedback do usuário sobre a spec
067 — "melhorou um pouco mas ainda dá pra organizar melhor".

## Contexto

Análise visual com dado real e volumoso (Neon Breach com os 3 arquétipos
match-based populados; Achievements Test com `queue_arena`+
`host_world_dropin` ambos com dado) revelou a causa raiz concreta do
"espaço desperdiçado": o `Grid` do Mantine estica todo card de uma linha
pra altura do vizinho mais alto (`Paper h="100%"` + comportamento padrão
de `align-items: stretch`). Quando um card tinha bastante conteúdo (stat
grid + donut) e o vizinho pouco (donut sozinho, ou um gráfico de linha com
1-3 pontos), o vizinho curto virava uma caixa vazia gigante só pra
"empatar" a altura — confirmado visualmente comparando as duas versões.

Achado secundário: "Resumo" (stat cards de partidas/vitórias/derrotas/
empates) e o donut "Vitórias × derrotas × empates" logo abaixo mostravam
os MESMOS 3 números duas vezes, em dois cards separados — mesmo padrão em
Battle Royale (`placement-distribution` já misturava os dois corretamente
num card só; só `summary`/`outcomes-donut` estavam separados
indevidamente).

## Comportamento

- `Grid` de cada seção SHALL usar `align="flex-start"` — cada card ocupa
  só a altura do que renderiza, sem esticar pra combinar com o vizinho.
- `summary` (Resumo) e `outcomes-donut` (Vitórias × derrotas × empates)
  SHALL ser um único card (stat grid + donut, mesmo tratamento já usado em
  `placement-distribution`/`mission-distribution`) — sem duplicar a
  mesma informação em dois cards.
- Gráficos de linha (`DailyLineChart`, reusado pelas 4 séries diárias)
  SHALL usar altura reduzida (170px, era 220px) — menos espaço
  desperdiçado quando há poucos pontos, ainda legível com muitos.
- Donuts (todos: resumo, placement, mission, session-health) SHALL usar
  tamanho reduzido e consistente (140px/thickness 20, era 160px/22) —
  peso visual mais compacto e uniforme entre os 4 usos.
- Um `Divider` SHALL separar visualmente cada seção da anterior (exceto a
  primeira) — antes as seções ficavam "coladas", só diferenciadas pelo
  cabeçalho pequeno.

## Critérios de Aceite

1. Jogo com múltiplos arquétipos populados (Neon Breach: 3 arquétipos
   match-based) -> cards de uma mesma linha NÃO têm mais altura idêntica
   forçada quando o conteúdo é diferente (validado visualmente: donut
   sozinho ao lado de gráfico esparso já não vira caixa vazia).
2. "Resumo" e o donut de vitórias/derrotas/empates aparecem num único
   card, sem duplicar os números.
3. Altura total da página cai perceptivelmente com o mesmo dado (Neon
   Breach: ~2380px -> ~2270px desktop; ~4150px -> ~3890px mobile).
4. Divisor visível entre seções consecutivas.
5. Zero erros de console nas 3 páginas testadas (Achievements Test, Neon
   Breach, Packet Arena), desktop e mobile.

## Fora de escopo

- Reordenar arquétipos ou mudar quais cards existem — só densidade/
  organização visual dos que já existem (spec 067 continua valendo pra
  isso).
- Compactar mais a tabela `participations-role`/`raw-stats`/
  `session-health` — já razoavelmente densas, sem redundância clara.
