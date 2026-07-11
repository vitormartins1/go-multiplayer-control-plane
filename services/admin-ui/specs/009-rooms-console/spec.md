# Spec: 009-rooms-console

**Serviço:** admin-ui
**Status:** Done (2026-07-04)
**Data:** 2026-07-04
**Parte de:** Rooms Console (Onda 1) — pares: `session-service/007-admin-session-control`, `admin-bff/008-rooms`

## Contexto

Transforma o painel de read-only em **console de gestão de salas**. Duas entregas:

1. Nova aba top-level **Rooms** — board global das sessões ao vivo (todos os jogos), com drill-down e a ação de **encerrar sala**. É o "control room" no estilo GameLift/PlayFab Game Manager.
2. **Saúde do fan-out** no Infrastructure — cards de entrega (publish/consume) do fan-out durável (ADR-0020).

A aba **Sessions** por-jogo (dentro de GameDetail, spec 007-sessions-tab) **permanece** como recorte por jogo; Rooms é a visão agregada.

## Comportamento

### Aba Rooms (nova, na nav lateral)

- THE SYSTEM SHALL adicionar **Rooms** na nav (ícone de sala/gamepad), rota `/rooms`.
- THE SYSTEM SHALL listar as **sessões ativas de todos os jogos** (`GET /api/rooms`), com **auto-refresh por polling** (mesmo padrão da aba Sessions: intervalo ~5s, para ao desmontar). Colunas: Jogo (nome), Match, Players (com handle), Server (address), Duração (viva, calculada do `openedAt`), Aberta em.
- THE SYSTEM SHALL permitir alternar para **fechadas** (histórico recente) via filtro, como a aba Sessions.
- WHEN o operador clica numa sala -> THE SYSTEM SHALL expandir o **detalhe** (players, serverId/address, tempos, motivo de fechamento se fechada).
- WHEN o operador aciona **Encerrar** numa sala ativa -> THE SYSTEM SHALL pedir **confirmação** (modal, mostra match/jogo) e então `POST /api/rooms/{matchId}/close`; no sucesso, a sala sai do board (ou vira fechada/`admin`); erro -> notificação.
- Estados vazio/carregando/erro tratados (sem placeholder piscando), como as telas atuais.

### Infrastructure — cards de saúde do fan-out

- THE SYSTEM SHALL adicionar um bloco **"Fan-out / entrega"** no Infrastructure com `GET /api/health/fanout`: publicações OK vs erro, consumos OK/dup/erro por sink. Erro/ausência de métrica -> estado degradado explícito (não quebra a tela). Segue o estilo dos cards de live metrics já existentes.

## Contratos de Interface (cliente)

- `api.rooms(state?)` -> `GET /api/rooms`; `api.closeRoom(matchId)` -> `POST /api/rooms/{matchId}/close`; `api.fanoutHealth()` -> `GET /api/health/fanout`.
- Nova página `pages/Rooms.tsx` (lazy, como as outras rotas — code-splitting, ver 008-ui-polish).
- Reuso: `StatusBadge`, padrão de polling e de linha expansível da `GameSessions.tsx` (extrair um helper se compensar).

## Casos de Teste / Verificação

- [ ] Rooms lista salas de jogos distintos com nome do jogo e handles; duração atualiza no polling
- [ ] filtro ativas/fechadas funciona
- [ ] Encerrar -> confirma -> some do board; cancelar não faz nada; erro notifica
- [ ] Infrastructure mostra os cards de fan-out; com o serviço fora, degrada sem quebrar
- [ ] rota `/rooms` é lazy (não entra no bundle de boot)

## Fora do escopo

- Ações além de encerrar (ex.: kick de player, mensagem à sala) — futuro.
- Gráfico temporal do fan-out (fica no Grafana; aqui é card instantâneo).
- Real-time via WebSocket (polling, como hoje).
