# Spec: 007-sessions-tab

**Serviço:** admin-ui
**Status:** Done (2026-07-03)
**Data:** 2026-07-03

## Contexto

Aba **Sessions** na página do jogo (`GameDetail`): ver as partidas do jogo — abertas e fechadas — para operação e debug (players da sala, server, se fechou e por quê). Consome `GET /api/games/{gameId}/sessions` (admin-bff `007-sessions-panel`).

## Comportamento

- Nova `Tabs.Tab` **Sessions** no `GameDetail` (ícone de server), painel `GameSessions.tsx`.
- **Tabela** (padrão visual das outras abas), mais recente primeiro:
  - **Match** (`matchId` truncado, tooltip com o inteiro; `sessionId` no tooltip também)
  - **Estado**: Badge — `open` verde ("ao vivo"), `closed` cinza; fechada mostra o **motivo** (`result` | `ttl`) no badge/tooltip
  - **Players**: contagem + handles (ou id truncado quando sem handle), ex. avatares/`Tooltip` como a casa já faz
  - **Server**: `serverAddress`
  - **Duração/tempos**: aberta → "há Xmin" (de `openedAt`); fechada → duração (`closedAt - openedAt`) e quando fechou
- **Polling**: recarregar a cada ~5s enquanto a aba está visível (é a tela de debug ao vivo); indicador discreto de última atualização.
- Filtro por estado (`todas | abertas | fechadas`) — client-side ou via `state=`, o que for mais simples com o hook da casa.
- **Empty state** explicando: sem partidas; e nota de que o histórico é em memória (some no restart do serviço).
- Erro do BFF → estado de erro padrão das outras abas.

## Casos de Teste / Aceite

- [x] Partida aberta aparece ao vivo (com o fluxo real rodando no data plane) e some/vira `closed` quando fecha
- [x] Motivo de fechamento visível (`result`/`ttl`)
- [x] Polling atualiza sem interação; sair da aba para o polling
- [x] Filtro estado funciona; empty state e erro ok
- [x] Visual consistente com as outras abas (Mantine, mesmo espaçamento/badges)

## Fora do escopo

- Ações (encerrar sessão, kick player) — evolução
- Push/websocket (polling basta)
- Visão da fleet Agones (contadores Ready/Allocated) — candidata a evolução desta aba
