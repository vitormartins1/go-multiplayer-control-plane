# Spec: 065-session-health-widget

**Serviço:** admin-ui
**Status:** Done
**Data:** 2026-07-13
**Parte de:** evolução do Analytics (item C) — consome `sessionHealth`
(analytics-service/013, admin-bff/027) pra responder as duas perguntas que
motivaram este item: "que % das sessões terminam abandonadas?" e "qual a
duração média de uma partida/mundo?".

## Contexto

`GameAnalyticsDetail` ganha `sessionHealth: SessionCloseStat[]` — a
distribuição por `closeReason` (contagem + duração), já filtrável por modo
(a tabela de origem nasce COM `mode_id`, sem a lacuna do item D que afeta
os outros 5 widgets cumulativos da spec 064). Falta um widget novo no
catálogo (padrão já estabelecido: `WidgetDef` com `icon`/`render`,
integrado ao `DashboardGrid` configurável da spec 062).

## Comportamento

- Novo widget `session-health` ("Saúde das sessões"): aparece quando
  `data.sessionHealth.length > 0`.
- Conteúdo: donut de distribuição por `closeReason` (rótulos amigáveis,
  mesmo padrão de `EVENT_LABELS`/`OUTCOME_LABELS`) + tabela com
  Motivo/Contagem/Duração média/% do total (dentro do escopo atual —
  mesma filosofia de "todos os modos" já usada nos outros widgets: se
  múltiplos modos estão combinados, o % é sobre a soma de todos).
- WHEN mais de um `modeId` distinto aparece nos dados recebidos (== "Todos
  os modos" selecionado) -> tabela ganha coluna "Modo" (mesmo padrão de
  `participations-role`).
- **Sem ícone de arquétipo fixo** (mesmo tratamento de `raw-stats`): a
  tabela de origem cobre TODOS os arquétipos (todo `closeSession` publica
  o evento, independente de arquétipo) — um widget cross-arquétipo. Fica
  SEMPRE elegível no catálogo, mesmo com um modo específico selecionado
  (não entra em `WIDGET_ARCHETYPE`).
- **Sem legenda de aviso** (diferente dos 5 widgets de `UNSCOPED_WIDGETS`
  da spec 064) — esta tabela JÁ nasce com `mode_id` corretamente
  escopado, o filtro por modo funciona de verdade aqui.
- Rótulos de motivo (fixos, mesmo padrão de `EVENT_LABELS`):
  `result` -> "Concluída", `ttl` -> "Expirou (TTL)", `admin` ->
  "Encerrada pelo operador", `abandoned` -> "Abandonada (crash)",
  `host_left` -> "Host saiu", `host_killed` -> "Host morto". SEM
  pré-classificar o que "conta como abandono" — cada motivo aparece
  identificado, o usuário decide o que agrupar mentalmente (mesma decisão
  já tomada no analytics-service: dado cru, sem rótulo de negócio embutido).

## Contratos de Interface

```ts
// lib/api.ts
export interface SessionCloseStat {
  modeId?: string;
  archetype?: string;
  closeReason: string;
  count: number;
  durationSumMs: number;
  avgDurationMs: number;
}

export interface GameAnalyticsDetail {
  // ... campos existentes
  sessionHealth: SessionCloseStat[];
}
```

```ts
// pages/GameAnalytics.tsx
const CLOSE_REASON_LABELS: Record<string, string> = {
  result: "Concluída",
  ttl: "Expirou (TTL)",
  admin: "Encerrada pelo operador",
  abandoned: "Abandonada (crash)",
  host_left: "Host saiu",
  host_killed: "Host morto",
};
```

## Critérios de Aceite

1. Jogo com `sessionHealth` presente -> widget "Saúde das sessões" aparece
   com donut + tabela; motivos traduzidos.
2. Jogo com sessões `host_world_dropin` fechadas por `host_left`/`ttl`/
   `host_killed` reais -> % e duração média corretos (validado contra o
   mesmo dado usado pra fechar a spec 013).
3. Selecionar um modo específico -> widget continua visível (não é
   filtrado pelo catálogo por arquétipo) e os números refletem SÓ aquele
   modo, sem legenda de aviso (diferente dos outros 5 widgets cumulativos).
4. Jogo sem nenhum `session_closed` processado ainda -> widget não
   aparece (mesmo padrão de gating por presença de dado dos outros
   widgets).

## Fora de escopo

- Fórmula de "% abandonada" pré-calculada — o usuário lê a tabela crua.
- Série temporal de fechamentos por dia — fora de escopo também no
  analytics-service (spec 013).
