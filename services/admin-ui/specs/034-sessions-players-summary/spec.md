# Spec: 034-sessions-players-summary

**Serviço:** admin-ui
**Status:** Aprovado e implementado (2026-07-08)
**Data:** 2026-07-08
**Parte de:** lote console-ux-2.

## Contexto

A coluna "Players" das listagens de sessão (Rooms e aba Sessions) renderiza avatar +
handle + badge de TODOS os jogadores na própria linha — com 6+ players (ranked 3v3,
royale de 8+) a linha estoura o layout. A lista completa já existe no painel expandido;
a linha precisa só de um resumo.

## Comportamento

- THE SYSTEM SHALL exibir na coluna Players da linha um **resumo compacto**: os
  avatares dos 3 primeiros jogadores sobrepostos (stack) + contador total
  (ex.: `⬤⬤⬤ 8`), com Tooltip listando os handles/ids de todos.
- WHEN a capacidade da sala for derivável do perfil já cacheado na página (Rooms:
  battle_royale `lobbySize`, coop `teamSize` — já usados nos badges hoje) → o contador
  vira `N/capacidade`. Sem perfil → só `N` (sem fetch novo).
- Sessões host_world continuam com `RoleSlots` na célula (já é compacto).
- O painel expandido segue exibindo a lista completa (avatar + handle + id + guest).

## Contratos de Interface (cliente)

- `components/PlayersSummary.tsx` (novo): stack de `PlayerAvatar` (máx. 3, overlap
  negativo) + `Text` contador + Tooltip com a lista.
- `Rooms.tsx` / `GameSessions.tsx`: linha usa `PlayersSummary`; expandido intacto.

## Casos de Teste / Verificação

- [x] sala de 8 players cabe numa linha sem quebrar; tooltip lista os 8
- [x] BR/coop mostram `N/cap`; queue_arena sem perfil mostra só `N`
- [x] expandido inalterado; `npm run build` limpo

## Fora do escopo

- Coluna nova de metadado (não adicionar ruído até sentirmos falta); fetch de perfil
  por modo na listagem; mudanças no BFF.
