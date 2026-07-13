# Spec: 062-configurable-analytics-dashboard

**Serviço:** admin-ui
**Status:** Done (2026-07-13) — validado em browser real (Playwright ad-hoc):
drag/reorder, hide, restaurar padrão e persistência entre reloads confirmados
com dado real (jogo neon-breach, 9 widgets incl. as 3 séries diárias novas e
o resumo de stats).
**Data:** 2026-07-13
**Parte de:** analytics-service/010+011 ("Analytics Fase 2/3", item 6 do backlog).

## Contexto

A aba Analytics por jogo (`GameAnalytics.tsx`, spec `admin-ui/054`) hoje
renderiza um conjunto FIXO de seções (cards de resumo + LineChart de
partidas/dia + Donuts + BarChart de participações), cada uma condicionada a
ter dado (`if (data.placement) ...`), em ordem fixa no código. Com a Fase 2
adicionando 3 séries diárias novas e um resumo de stats, o número de seções
possíveis cresce (~9 pro caso mais rico, host_world_dropin) — o operador não
tem como esconder o que não usa nem reordenar pro que importa mais pro jogo
dele.

## Comportamento

- Cada seção hoje fixa (e as novas da Fase 2) vira um **widget** com um id
  estável: `summary` (StatCard row), `queue-daily`, `outcomes-donut`,
  `participations-role`, `placement-distribution`, `placement-daily`,
  `mission-distribution`, `mission-daily`, `participation-daily`,
  `raw-stats`.
- Um widget só entra no CATÁLOGO DISPONÍVEL do jogo se a condição de dado de
  hoje for satisfeita (ex.: `placement-distribution` só existe se
  `data.placement` não for nil) — igual ao comportamento atual, só que agora
  descrito centralizadamente por widget em vez de JSX condicional espalhado.
- THE SYSTEM SHALL permitir ao operador, em um **modo de edição** (toggle):
  arrastar/redimensionar/reordenar os widgets disponíveis e esconder/mostrar
  cada um.
- O layout (posição, tamanho, visibilidade, ordem) é persistido no
  `localStorage` do navegador, **por jogo** (chave inclui o `gameId`) — sem
  backend novo (sem evidência de necessidade de sincronizar entre
  dispositivos/operadores ainda; ver Fora de escopo).
- IF não há layout salvo pro jogo (primeira visita) -> THE SYSTEM SHALL usar
  um layout padrão (ordem equivalente à atual, todos os widgets disponíveis
  visíveis).
- Um botão "Restaurar padrão" limpa o layout salvo e volta ao default.
- Fora do modo de edição, os widgets renderizam read-only na posição/tamanho
  salvos — sem handles de drag/resize visíveis (não polui a leitura do dia a
  dia).
- Um widget que DEIXA de ter dado (ex.: jogo mudou de arquétipo) some da
  renderização mas continua na config salva (não perde o layout customizado
  se o dado voltar).

## Decisão técnica

**Biblioteca:** [`react-grid-layout`](https://github.com/react-grid-layout/react-grid-layout)
— 2,97M downloads/semana, 22,3k stars, nativa React (sem jQuery), suporta
drag+resize+breakpoints responsivos, mantida ativamente. Comparada com
`gridstack.js` (wrapper React sobre core vanilla-JS, ~336 downloads/semana):
react-grid-layout é a escolha dominante do ecossistema e integra bem com
Mantine (cada célula do grid é só um componente React normal — os
`FormSection` com gráfico que já existem viram o conteúdo de cada widget,
sem reescrever os gráficos).

**Persistência:** `localStorage`, chave `analytics-dashboard-layout:{gameId}`,
valor = array `{i: widgetId, x, y, w, h}[]` (formato nativo do
react-grid-layout, serializa direto) + `hidden: string[]` (ids escondidos).
Cliente-only por escolha deliberada (YAGNI): não há caso real hoje pedindo
layout compartilhado entre operadores/dispositivos — se aparecer, migra pra
um endpoint de config no admin-bff sem quebrar o formato (é aditivo).

## Casos de Teste / Verificação

- [x] Todos os widgets com dado aparecem no layout padrão, na primeira visita
      (sem localStorage prévio)
- [x] Modo de edição: arrastar/redimensionar/esconder um widget persiste no
      localStorage; recarregar a página mantém o layout customizado
- [x] "Restaurar padrão" limpa o localStorage e volta ao layout default
- [x] Widget sem dado (arquétipo não aplicável) não aparece no catálogo, mas
      não quebra o layout salvo dos demais
- [x] Trocar de jogo (gameId diferente) usa o layout daquele jogo
      especificamente, não vaza do jogo anterior
