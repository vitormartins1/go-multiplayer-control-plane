# Spec 054 — Fase 1 do dashboard de Analytics por jogo (gráficos)

> Status: **implementada** (Fase 1 de um plano em 3 fases, aprovado pelo
> Vitor em 2026-07-10; Fases 2/3 ficam pra depois)
> Fatia: pequena/aditiva — só admin-ui (+ o achado de backend, spec
> analytics-service/008); usa `@mantine/charts`, já instalado e usado na
> aba geral (`Analytics.tsx`).

## Contexto

Levantamento prévio (pesquisa, não implementação) mapeou o estado atual:
- Backend (analytics-service) é **por arquétipo**, não genérico — 4
  caminhos de ingestão, 6 tabelas próprias, cada arquétipo com seu rollup.
- Série temporal (`game_daily`) só existe pra `queue_arena`; os outros 3
  arquétipos são contadores cumulativos sem dimensão de tempo.
- `stats` brutos (kills/deaths) trafegam no envelope mas o analytics-service
  descarta tudo — não é tocado nesta fase.
- A aba de Analytics **do jogo** (`GameAnalytics.tsx`) tinha ZERO gráfico —
  só `StatCard` + uma tabela (HWD), tudo condicional por arquétipo. A aba
  **geral** já usa `@mantine/charts` (`LineChart`) — biblioteca aprovada,
  sem necessidade de trocar (cobre Line/Bar/Area/Pie/Donut/Radar, já
  integrada ao tema).

Plano acordado em 3 fases; esta spec é só a **Fase 1**: usar dados que já
vêm no DTO mas não eram visualizados, sem mudança de schema.

## Comportamento (EARS)

1. Quando o jogo tiver série diária (`data.daily`, hoje só `queue_arena`), a
   aba DEVE renderizar um `LineChart` "Partidas por dia" — o dado já vinha
   da API e não era exibido em lugar nenhum.
2. Quando o jogo tiver vitórias/derrotas/empates (`wins+losses+draws > 0`),
   a aba DEVE renderizar um `DonutChart` de distribuição, complementando os
   `StatCard` já existentes (não substitui).
3. Quando o jogo tiver `placement` (battle_royale), a aba DEVE renderizar um
   `DonutChart` de 1º lugar / Top 10 / Resto, além dos `StatCard`.
4. Quando o jogo tiver `mission` (coop_mission), a aba DEVE renderizar um
   `DonutChart` de missões cumpridas/falhas, além dos `StatCard`.
5. Quando o jogo tiver `roleParticipations` (host_world_dropin), a aba DEVE
   renderizar um `BarChart` horizontal (papel×evento → total) acima da
   tabela existente; a tabela ganha uma coluna **Desfecho** (outcome, spec
   analytics-service/008) que antes não estava disponível.
6. Nenhum `StatCard`/tabela existente é removido — os gráficos são aditivos.

## Critérios de aceite

- [x] `GameAnalytics.tsx` renderiza `LineChart`/`DonutChart`/`BarChart` do
      `@mantine/charts`, condicionados exatamente pelas mesmas guardas que
      já existiam (`data.daily.length`, `data?.placement`, `data?.mission`,
      `participations.length`).
- [x] Validado com dado real (não só mock): Packet Arena (queue_arena +
      placement + mission simultâneos) e Neon Breach (participação HWD via
      SNS de teste, limpa depois) — todos os gráficos renderizaram
      corretamente com as guardas corretas.
- [x] Build verde (`tsc` + vite build).
- [x] Backend: `outcome` exposto no DTO de participação (achado + fechado
      junto, spec analytics-service/008 — sem isso a tabela/gráfico de HWD
      não teriam a dimensão de desfecho).

## Fora de escopo (fica pras Fases 2/3, gated por evidência)

- Fase 2: estender granularidade diária pros outros 3 arquétipos (schema
  novo); capturar `stats` brutos no analytics-service.
- Fase 3: dashboard configurável por widget (customização de verdade).
- Qualquer mudança na aba de Analytics **geral** (fora do escopo combinado
  — "focar primeiro na do jogo").
