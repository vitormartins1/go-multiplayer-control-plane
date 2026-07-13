# Spec: 061-session-history-ui

**Serviço:** admin-ui
**Status:** Done
**Data:** 2026-07-13
**Parte de:** session-service/025-session-history (Fatia 4).

## Contexto

O BFF (admin-bff/023) já expõe o histórico persistido de sessões fechadas
(spec 025): `GET /api/games/{gameId}/sessions?state=closed&limit=&before=`
(mudou de fonte, ring → store, na MESMA rota) e `GET /api/players/{playerId}
/sessions?gameId=&limit=&before=` (novo). Falta a UI: uma aba **History** na
página do player e paginação "carregar mais" na aba Sessions do jogo.

## Comportamento

- **Aba Sessions do jogo (`GameSessions`)**: as fechadas passam a vir do
  endpoint persistido (`api.closedGameSessions`), paginado — botão "Carregar
  mais fechadas" usa o `nextBefore` da página como cursor. As ATIVAS
  continuam do mapa in-memory (`api.gameSessions`, sem `state`), com o
  polling de 5s de sempre. Visual da tabela/expand mantido.
- IF o store persistido responder 503 `history_unavailable` (Postgres do
  histórico fora) → a aba cai pro `data.closed` do ring in-memory (já vinha
  no `gameSessions` de qualquer forma), com um `Alert` amarelo avisando —
  sem "carregar mais" nesse modo (o ring não pagina).
- **Aba History no `PlayerDetail`** (`PlayerHistory`, componente novo): nova
  aba logo após Achievements, escopada ao jogo selecionado no seletor do
  header (mesmo padrão de `PlayerAchievements`). Sem jogo selecionado → aviso
  "Selecione um jogo no topo". Tabela: Quando (`closedAt`), Modo, Papel do
  player (lido de `participants`, cruzando pelo `playerId`), Motivo
  (`closeReasonLabel`), Duração (`formatDuration`), Participantes
  (`PlayersSummary`, reaproveitado da aba Sessions). "Carregar mais" com o
  mesmo padrão de cursor. Sem sessões → "Nenhuma sessão fechada deste
  jogador neste jogo ainda."

## Casos de Teste / Verificação (manual, Playwright ad-hoc)

- [x] Aba Sessions do jogo mostra as fechadas do store persistido; expandir
      uma sessão fechada não gera erro
- [x] Aba History aparece logo após Achievements (ícone de relógio); com jogo
      selecionado, mostra a sessão fechada com Papel correto (ex.: host)
- [x] Console sem erros novos (404 pré-existente de cloud-save sem save
      cadastrado, não relacionado)
