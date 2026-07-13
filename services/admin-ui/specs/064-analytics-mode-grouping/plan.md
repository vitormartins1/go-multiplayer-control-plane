# Plan — 064 agrupamento por arquétipo/modo no Analytics

## Decisões técnicas

1. **Badge de ícone no `WidgetDef`, não seção física no grid** — preserva o
   drag/resize/reorder livre da spec 062 (validado ao vivo, o usuário
   gostou); reorganizar fisicamente em blocos por arquétipo exigiria um
   segundo sistema de layout (seções fixas + grid livre dentro de cada
   uma), complexidade desproporcional ao problema (o usuário quer
   identificar de relance o que é o quê, não perder a liberdade de arranjo).
2. **Mapeamento widget→arquétipo como tabela estática no componente**
   (`WIDGET_ARCHETYPE`), não um campo vindo do backend — a associação é
   1:1 e fixa pelo próprio catálogo de widgets (cada widget já é
   condicionado a um campo do `GameAnalyticsDetail` que só existe pra um
   arquétipo); inventar isso no backend seria acoplamento desnecessário
   pra um dado puramente de apresentação.
3. **Seletor de modo move de dentro do widget pra cima da página** —
   elimina duplicação (o padrão já existe, só precisa subir de nível) e
   troca de filtro client-side (sobre dado já carregado) pra filtro real
   no backend (`useAsync` ganha `selectedMode` nas deps, refaz o fetch).
4. **Filtragem do catálogo por arquétipo quando modo específico
   selecionado** (não só "deixar os vazios sumirem sozinhos") — decisão
   crítica: os 5 widgets cumulativos (`summary`/`queue-daily`/
   `outcomes-donut`/`placement-distribution`/`mission-distribution`) vêm de
   tabelas SEM `mode_id` (item D, ainda não corrigido) — filtrar só pelos
   campos mode-scoped deixaria esses 5 aparecerem com número GLOBAL sob um
   filtro de modo específico, o que é mais enganoso que não filtrar nada.
   Filtrar o catálogo por arquétipo evita esse caso raiz.
5. **Legenda de aviso nos widgets cumulativos, não escondê-los quando o
   arquétipo bate mas o dado não é filtrável** — ex.: jogo com 2 modos
   `queue_arena`, um selecionado: `summary`/`outcomes-donut` são do
   arquétipo certo mas ainda somam os 2 modos (item D). Esconder perderia
   informação real (o total ainda é válido, só não está recortado);
   melhor avisar que resumir errado ou fingir que filtrou.

## Arquivos por camada

- `lib/api.ts`:
  - `gameAnalytics(gameId, opts?: { modeId?: string }, signal?)` — mesmo
    padrão de `gameLeaderboard` (URLSearchParams no call site).
  - `GameAnalyticsDetail` ganha `modeId?: string` (eco do filtro).
- `lib/archetypeMeta.ts`: SEM MUDANÇA (`ARCHETYPE_META` já existe, só
  reaproveitado).
- `components/DashboardGrid.tsx`:
  - `WidgetDef` ganha `icon?: Icon` opcional.
  - Render do cabeçalho do card: `{widget.icon && <widget.icon size={14} />}`
    ao lado do título, sem mudar layout/persistência (campo é só
    apresentação, não entra em `WidgetLayoutItem`/localStorage).
- `pages/GameAnalytics.tsx` (reescrita pontual, não total):
  - `WIDGET_ARCHETYPE: Record<string, MultiplayerProfile["archetype"]>`
    (novo, tabela estática).
  - `UNSCOPED_WIDGETS: Set<string>` (novo, tabela estática — os 5
    widgets cumulativos do achado).
  - `selectedMode` sobe de "estado interno usado só pelo widget
    participations-role" pra "estado que dirige o fetch" — passa a
    integrar as deps do `useAsync`.
  - `useAsync` chama `api.gameAnalytics(gameId, { modeId: selectedMode ===
    ALL_MODES ? undefined : ... })`.
  - Novo bloco de UI acima do `DashboardGrid`: `<Select label="Modo" ...>`
    (só se `modes.length > 1`), reaproveitando os mesmos sentinelas
    `ALL_MODES`/`NO_MODE` já existentes.
  - Construção do array `widgets` (dentro do `useMemo`): cada `push` ganha
    `icon: ARCHETYPE_META[...]?.icon` (via `WIDGET_ARCHETYPE`); no fim do
    `useMemo`, se `selectedMode !== ALL_MODES`, filtra o array final pelo
    arquétipo do modo selecionado (mais `raw-stats`, sempre elegível).
  - Legenda de aviso: pequeno `<Text size="xs" c="dimmed">` dentro do
    `render()` dos 5 widgets de `UNSCOPED_WIDGETS`, condicional a
    `selectedMode !== ALL_MODES && modes.length(daquele arquétipo) > 1`.
  - `participations-role`: remove o `Select` interno e a lógica de
    filtragem client-side (`allParticipations.filter(...)`) — usa
    `data.roleParticipations` direto (já vem filtrado pelo backend quando
    aplicável). Mantém a lógica de mostrar coluna "Modo" só quando há mais
    de 1 modo distinto NOS DADOS RECEBIDOS (que agora só acontece com
    "Todos os modos" selecionado, já que um modo específico não teria
    variação).
- Testes: sem suite automatizada de componente neste projeto (confirmado
  em specs anteriores — validação é `tsc --noEmit` + Playwright ad-hoc).

## Ordem de implementação

`lib/api.ts` (client + tipo) → `DashboardGrid.tsx` (ícone no card) →
`GameAnalytics.tsx` (tabelas estáticas → seletor de página → wiring do
fetch → filtragem do catálogo → legendas) → `tsc --noEmit` → build →
validação via Playwright ad-hoc contra dado real (Achievements Test: 2
arquétipos; Packet Arena: 2 modos queue_arena com `orbs`, prova a legenda
de aviso E o filtro funcionando lado a lado).
