# Plan: 009-rooms-console

**Spec:** specs/009-rooms-console/spec.md
**Data:** 2026-07-04

## Abordagem técnica

Reusa o padrão da `GameSessions.tsx` (polling 5s via `useAsync` + `setInterval`, linha expansível, duração ao vivo) para o board global; adiciona coluna **Jogo** e a ação **Encerrar** (Modal de confirmação do `@mantine/core` — não há ModalsProvider — + `notifications`). Cards de fan-out no Infrastructure via `useAsync(api.fanoutHealth)`.

## Arquivos

- `src/lib/api.ts`:
  - `GameSession` ganha `gameId?`, `gameName?`; `closeReason` inclui `"admin"`.
  - `FanoutHealth` (campos `*number | null`).
  - `api.rooms(state?)`, `api.closeRoom(matchId)`, `api.fanoutHealth()`.
- `src/lib/format.ts` (novo) — extrai `truncate` + `formatDuration` (hoje locais na GameSessions), reusados por Rooms.
- `src/pages/GameSessions.tsx` — passa a importar de `lib/format` (remove as cópias locais).
- `src/pages/Rooms.tsx` (novo, lazy) — board global: filtro Todas/Abertas/Fechadas, polling, linha expansível (coluna Jogo + detalhe), botão **Encerrar** nas abertas → `Modal` confirma → `api.closeRoom` → `reload` + notificação.
- `src/App.tsx` — item de nav **Rooms** (`IconDoorEnter`/afim, rota `/rooms`) + `Route` lazy.
- `src/pages/Overview.tsx` — bloco **"Fan-out / entrega"**: cards publish ok/erro, consume ok/dup/erro (nulo → "—"/degradado); `useAsync(api.fanoutHealth)`.

## Decisões

- **Board = ativas por default** (`state=open`? não — busca tudo e filtra no cliente, como a GameSessions faz, mas o board prioriza abertas no topo). Mantém 1 request; filtro no cliente.
- **Encerrar** só em sessão `open`; confirmação obrigatória (ação destrutiva). Erro → `notifications` vermelho; sucesso → `reload` (a sala vira fechada/`admin` ou some conforme o filtro).
- **Extrair `format.ts`** em vez de duplicar (2 funções puras) — evita divergência com a GameSessions.
- **Fan-out cards** degradam com nulo/erro (503) sem quebrar a tela, como o resto do Infrastructure.
- Rota **lazy** (code-splitting, 008-ui-polish).

## Ordem

1. `lib/format.ts` + ajustar GameSessions. 2. `api.ts` (tipos + métodos). 3. `Rooms.tsx`. 4. `App.tsx` (nav+rota). 5. `Overview.tsx` (cards). 6. `npm run build` (typecheck) + verificação visual.

## Riscos

- **closeReason "admin"** no union → a GameSessions exibe motivo; incluir o rótulo "encerrada pelo operador".
- **Polling + Modal aberto**: o reload de 5s não deve fechar o Modal (o Modal é estado local, independente do `data`). OK.
- **Duração ao vivo** recalcula no render; o polling de 5s já provoca re-render suficiente.
