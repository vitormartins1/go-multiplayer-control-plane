# Spec: 021-battle-royale-console

**Serviço:** admin-ui · **Status:** Rascunho (aguardando aprovação) · **Data:** 2026-07-07
**ADR:** 0023 — arquétipo `battle_royale` · **Roadmap:** BR4 (surfacing) + fecha o gap da BR0-UI
**Depende de:** BR0 (`game-registry/007` — perfil BR no servidor), BR1 (lobby/squads), BR3 (`analytics/005` — distribuição), `admin-bff/015` (propaga `placement`)

## Contexto

O `battle_royale` existe no backend (perfil BR0, lobby BR1, resultado BR2, sinks BR3) mas **não existe no console** — o admin-ui só conhece `queue_arena` e `host_world_dropin`. Esta fatia traz o arquétipo ao console em três frentes:

1. **Declarar** o perfil `battle_royale` (fecha o gap da BR0-UI): o form de multiplayer ganha o ramo `battle_royale`.
2. **Analytics:** bloco condicional com a distribuição de colocação (BR3).
3. **Rooms:** a sala de um jogo `battle_royale` mostra o lobby (N/`lobbySize`) e o modo (solo/squad).

Espelha os padrões já existentes: o form de perfil (`MultiplayerSettings.tsx`, host_world_dropin/spec 020), o bloco condicional de analytics (participações, spec 014) e o board de Rooms (spec 020).

## Comportamento

### 1. Perfil `battle_royale` no form (`MultiplayerSettings.tsx`)

- WHEN o operador escolhe o arquétipo `battle_royale` → THE SYSTEM SHALL exibir os campos: **lobbySize** (≥ 2 e ≥ 2×teamSize), **teamSize** (≥ 1; 1 = solo/FFA, 2 = duo, 4 = squad), **respawnMode** (`one_life`/`teammate_revive`/`arena_1v1`/`reboot_timer`) e **scoringStats** (tags, mesma validação `^[a-z][a-z0-9_]*$`), com tooltips explicativos.
- WHEN salva → THE SYSTEM SHALL enviar `{ archetype: "battle_royale", topology, params: { lobbySize, teamSize, respawnMode, scoringStats } }` via `setMultiplayerProfile` (passthrough raw do bff → game-registry valida).
- WHEN carrega um perfil `battle_royale` existente → popula o form. THE SYSTEM SHALL desabilitar salvar se `lobbySize < 2×teamSize` ou houver stat inválido (validação de UX; o servidor é a autoridade final).
- `queue_arena`/`host_world_dropin` **sem regressão** (mesmo load/save/dispatch).

### 2. Analytics — distribuição de colocação (`GameAnalytics.tsx`)

- WHEN o detalhe do jogo traz `placement` (bff/015) → THE SYSTEM SHALL exibir um bloco "Distribuição de colocação": os baldes **1º lugar** (`winners`), **Top 10** (`topN`), **Resto** (`rest`), o total de participações (`entries`) e a **sobrevivência média** (`avgSurvivedMs`, formatada legível — ex.: `m s`). Só aparece quando `placement` existe (jogo não-BR não regride).

### 3. Rooms — lobby e modo (`Rooms.tsx`)

- WHEN uma sessão pertence a um jogo com perfil `battle_royale` → THE SYSTEM SHALL exibir na sala o **lobby** `N/lobbySize` (N = nº de players da sessão) e o **modo** (`teamSize === 1` → "Solo/FFA"; senão "Squads de {teamSize}"). Busca o perfil por jogo (cacheado por `gameId`; reusa `getMultiplayerProfile`).
- IF o perfil não é `battle_royale` (ou indisponível) → a sala renderiza como hoje (sem o rótulo BR), sem regressão.

## Contratos de Interface

- `src/lib/api.ts`: `BattleRoyaleParams { lobbySize, teamSize, respawnMode, scoringStats }`; `battle_royale` na union `MultiplayerProfile`; `PlacementDistribution { winners, topN, rest, entries, avgSurvivedMs }` e `placement?` no retorno de `gameAnalytics`.
- `MultiplayerSettings.tsx`: `BR_DEFAULTS`, `buildBattleRoyaleProfile`, `BattleRoyaleForm`; archetype state e o Select ganham `battle_royale`; load/save dispatcham o novo ramo.
- `GameAnalytics.tsx`: bloco de distribuição (helper de formatação de duração).
- `Rooms.tsx`: fetch+cache de perfis por `gameId`; render do rótulo BR por sala.

## Casos de Teste / Verificação

- Sem suíte de testes no admin-ui (só `npm run build` = `tsc --noEmit && vite build`). THE SYSTEM SHALL passar no typecheck/build. Verificação manual (smoke): declarar um perfil BR, ver o bloco de analytics quando há dados, e o rótulo de lobby/modo no Rooms de um jogo BR.

## Fora do escopo

- **Vivos/eliminados ao vivo** no Rooms (tracking incremental) — follow-up (decisão #1 do roadmap).
- Gráfico rico (barras) da distribuição — um bloco de números/StatCards basta; charts é polimento.
