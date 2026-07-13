# Spec: 064-analytics-mode-grouping

**Serviço:** admin-ui
**Status:** Done
**Data:** 2026-07-13
**Parte de:** evolução do Analytics (item B da ordem A→B→C→D decidida com o
usuário) — consome o filtro `?modeId=` entregue pelo item A
(analytics-service/012 + admin-bff/026).

## Contexto

Achado do usuário ("os dados que já têm lá meio bagunçados"), confirmado ao
ler `GameAnalytics.tsx`: a página mostra os 10 widgets do catálogo (spec
062) num grid único e achatado, sem nenhuma indicação visual de a qual
arquétipo/modo cada um pertence. Um jogo com mais de um arquétipo (ex.:
Achievements Test: `queue_arena` + `host_world_dropin`) mostra "Partidas por
dia" (queue_arena) ao lado de "Eventos de participação por dia"
(host_world_dropin) sem nenhuma separação — o usuário precisa já saber de
cor o que cada card significa.

Hoje só o widget `participations-role` tem seletor de modo — e é
client-side puro (filtra `data.roleParticipations` já carregado, sem tocar
o backend). Os outros 9 widgets são sempre globais.

**Achado adicional confirmado nesta revisão** (relevante pro design desta
spec): nem toda série é filtrável por modo — a spec 012 do
analytics-service só cobre `PlacementDaily`/`MissionDaily`/
`ParticipationDaily`/`Stats`/`RoleParticipations`. Os campos cumulativos
(`Matches`/`Wins`/`Losses`/`Draws`/`AvgScore`/`UniquePlayers`/`Daily`/
`Placement`/`Mission`) vêm de tabelas SEM `mode_id` (`game_rollup`,
`game_daily`, `game_placement_rollup`, `game_mission_rollup` — item D,
ainda não corrigido) e **continuam globais mesmo com `?modeId=` aplicado**.
Isso afeta os widgets `summary`, `queue-daily`, `outcomes-donut`,
`placement-distribution` e `mission-distribution`: mesmo filtrando por um
modo específico, esses 5 widgets somam TODOS os modos daquele arquétipo (não
só o modo escolhido). A spec precisa comunicar isso, não escondê-lo.

## Comportamento

### Badge de arquétipo por widget (sempre, com ou sem filtro)

- Cada widget do catálogo SHALL exibir um ícone de arquétipo no cabeçalho do
  card (reaproveita `ARCHETYPE_META[archetype].icon`, mesmo ícone já usado
  nos badges de modo do header do GameDetail — sem cor nova, mesma decisão
  documentada em `archetypeMeta.ts` de não competir com a paleta do canvas
  de fluxo). Mapeamento fixo:
  - `summary`, `queue-daily`, `outcomes-donut` -> `queue_arena`
  - `participations-role`, `participation-daily` -> `host_world_dropin`
  - `placement-distribution`, `placement-daily` -> `battle_royale`
  - `mission-distribution`, `mission-daily` -> `coop_mission`
  - `raw-stats` -> SEM badge (cross-arquétipo por natureza — qualquer modo
    pode reportar `scoringStats`; forçar um ícone seria enganoso).
- Isso já ajuda a "dado bagunçado" mesmo sem nenhum filtro ativo: o usuário
  identifica de relance a que arquétipo cada card pertence, sem abrir nada.

### Seletor de modo no nível da página (não mais só dentro de um widget)

- WHEN o jogo tem mais de 1 modo cadastrado -> THE SYSTEM SHALL mostrar um
  seletor "Modo" acima do grid de widgets (não mais escondido dentro do
  card de participações), com opções "Todos os modos" (default) + uma por
  modo (rótulo = `displayName` ou `modeId`, com o mesmo ícone de arquétipo
  do badge acima — mesmo padrão visual dos badges do header do GameDetail).
- WHEN o jogo tem 1 modo só (ou nenhum) -> seletor NÃO aparece (mesma regra
  já usada hoje pro seletor interno do `participations-role`, generalizada
  pra página inteira).
- Trocar o modo SHALL refazer a busca no admin-bff com `?modeId=` (troca de
  client-side-filter pra filtro real no backend, spec 012) — `useAsync`
  ganha `selectedMode` nas deps.

### Filtragem do catálogo por modo selecionado

- WHEN um modo específico está selecionado -> THE SYSTEM SHALL mostrar SÓ
  os widgets cujo arquétipo bate com o arquétipo do modo escolhido (usando
  o mapeamento da seção anterior) + `raw-stats` (sempre elegível, já vem
  corretamente filtrado pelo backend graças à spec 012).
- WHEN "Todos os modos" está selecionado (default) -> catálogo completo,
  comportamento atual (gating por presença de dado, inalterado).
- Widgets cujo dado-fonte NÃO é filtrável por modo (`summary`,
  `queue-daily`, `outcomes-donut`, `placement-distribution`,
  `mission-distribution` — ver achado da revisão) SHALL exibir uma legenda
  discreta ("dado agregado de todos os modos deste arquétipo — filtro por
  modo ainda não cobre este resumo") QUANDO um modo específico estiver
  selecionado E o jogo tiver mais de um modo daquele arquétipo. Não
  esconder o widget nem fingir que o número mudou — só avisar.
- Widgets já cobertos pelo filtro (`participations-role`,
  `participation-daily`, `placement-daily`, `mission-daily`, `raw-stats`)
  NÃO precisam de legenda — o número já reflete o modo escolhido de
  verdade.

### Simplificação do `participations-role`

- O `Select` interno de modo (client-side, filtrando `roleParticipations`
  já carregado) SHALL ser removido — superado pelo seletor de página, que
  já filtra no backend. A coluna "Modo" da tabela interna continua
  aparecendo quando o jogo tem mais de um modo `host_world_dropin` E
  "Todos os modos" está selecionado (senão é óbvio, é só aquele modo).

## Contratos de Interface

```ts
// lib/api.ts — mesmo padrão de gameLeaderboard (URLSearchParams montado no
// call site, sem helper novo)
gameAnalytics: (gameId: string, opts?: { modeId?: string }, signal?: AbortSignal) => {
  const q = new URLSearchParams();
  if (opts?.modeId) q.set("modeId", opts.modeId);
  const qs = q.toString();
  return getJSON<GameAnalyticsDetail>(
    `/api/analytics/games/${encodeURIComponent(gameId)}${qs ? `?${qs}` : ""}`,
    signal,
  );
},

export interface GameAnalyticsDetail {
  // ... campos existentes
  modeId?: string; // eco do filtro aplicado (admin-bff/026); ainda não existe no tipo do admin-ui, só no domain do admin-bff
}
```

```ts
// pages/GameAnalytics.tsx — mapeamento widget -> arquétipo (novo, fixo)
const WIDGET_ARCHETYPE: Record<string, MultiplayerProfile["archetype"]> = {
  summary: "queue_arena", "queue-daily": "queue_arena", "outcomes-donut": "queue_arena",
  "participations-role": "host_world_dropin", "participation-daily": "host_world_dropin",
  "placement-distribution": "battle_royale", "placement-daily": "battle_royale",
  "mission-distribution": "coop_mission", "mission-daily": "coop_mission",
};
// raw-stats fica de fora do mapa (sem badge, sempre elegível).

// widgets cujo dado-fonte não respeita modeId (item D, analytics-service) —
// usado só pra decidir se mostra a legenda de aviso.
const UNSCOPED_WIDGETS = new Set(["summary", "queue-daily", "outcomes-donut", "placement-distribution", "mission-distribution"]);
```

```tsx
// components/DashboardGrid.tsx — WidgetDef ganha o ícone opcional (renderizado
// no cabeçalho do card, ao lado do título; sem mudança de layout/persistência).
export interface WidgetDef {
  id: string;
  title: string;
  defaultH: number;
  icon?: Icon; // spec 064 — badge de arquétipo
  render: () => React.ReactNode;
}
```

## Critérios de Aceite

1. Jogo com 2+ arquétipos (Achievements Test) -> cada widget mostra o ícone
   do arquétipo certo no card, sem filtro nenhum ativo.
2. Jogo com 2+ modos -> seletor "Modo" aparece acima do grid; jogo com 1
   modo -> seletor não aparece (regressão zero pra maioria dos jogos hoje).
3. Selecionar um modo `host_world_dropin` específico -> só
   `participations-role`/`participation-daily`/`raw-stats` aparecem (mais
   `summary`/`queue-daily`/`outcomes-donut`/`placement-*`/`mission-*` FORA,
   por não serem do arquétipo); números batem com a API filtrada
   (`?modeId=`), validado contra o mesmo dado real do item A (Packet
   Arena: `orbs` isolado por modo em `raw-stats`).
4. Jogo `queue_arena` com 2+ modos, um selecionado -> `summary`/
   `outcomes-donut` aparecem com a legenda de aviso (dado ainda global,
   item D); `raw-stats` filtra de verdade, sem legenda.
5. Voltar pra "Todos os modos" -> catálogo completo, comportamento
   idêntico ao atual (regressão zero).
6. Layout salvo (spec 062, drag/resize/hide) continua funcionando —
   trocar de modo só muda QUAIS widgets aparecem (subconjunto do catálogo
   salvo), não reseta posição/tamanho dos que continuam visíveis.

## Fora de escopo

- **Corrigir os widgets não-filtráveis** (`summary`/`queue-daily`/
  `outcomes-donut`/`placement-distribution`/`mission-distribution`) pra
  respeitarem `modeId` de verdade — isso é o item D (mode_id nos rollups
  cumulativos), próximo item da ordem combinada com o usuário. Esta spec só
  avisa honestamente da limitação, não a resolve.
- **Ingestão de session history** (item C) — spec separada, não depende
  desta.
- **Reorganização física em seções/grupos no grid** (ex.: um bloco visual
  por arquétipo) — descartado em favor do badge de ícone por widget, que
  preserva a liberdade de drag/reorder/resize já entregue e validada na
  spec 062 sem introduzir um segundo sistema de layout.
