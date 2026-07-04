# Tasks: 009-rooms-console

**Plan:** specs/009-rooms-console/plan.md
**Data:** 2026-07-04

- [x] `src/lib/format.ts` — extrai `truncate` + `formatDuration`; `GameSessions.tsx` importa de lá
- [x] `src/lib/api.ts` — `gameId?`/`gameName?` + `"admin"` no `GameSession`; `FanoutHealth`; `api.rooms`/`closeRoom`/`fanoutHealth`
- [x] `src/pages/Rooms.tsx` — board global (polling, filtro, coluna Jogo, linha expansível, Encerrar + Modal confirma + notificação)
- [x] `src/App.tsx` — nav **Rooms** + rota lazy `/rooms`
- [x] `src/pages/Overview.tsx` — bloco "Fan-out / entrega" (cards via `api.fanoutHealth`, degrada com nulo/erro)
- [x] `npm run build` (typecheck limpo; chunk Rooms lazy)
- [x] verificação visual (board/close/cards) — manual, requer a stack completa de pé
