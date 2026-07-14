# Spec: 088-leaderboard-handles-player-id-emptystate

**Serviço:** admin-bff, admin-ui
**Status:** Implemented
**Data:** 2026-07-14
**Nota de processo:** escrita depois da implementação — mesmo caso das
specs 071 a 087. Três achados independentes do design review pedido pelo
Vitor (`/frontend-design:frontend-design`), tratados juntos por serem
pequenos e não terem decisão técnica nova.

## Contexto

Review de design encontrou 3 problemas pontuais:

1. **Leaderboard mostrava UUID cru** (`cba7c65b…`) em vez do handle do
   jogador, enquanto toda a outra superfície do console (Players,
   Sessions, Achievements) já resolve pra `Ana_Riva` etc.
2. **UUID completo do jogador** aparecia em `Text size="sm"` logo abaixo
   do nome no header do Player detail — peso visual alto pra um dado
   secundário, sem forma de copiar.
3. **Estados vazios** (Rooms sem sala, Player sem jogo, Games sem jogo,
   Sessions sem partida) eram só uma linha de texto solta — funcionavam,
   mas sem nenhum traço visual do resto do console.

## Decisões

### 1. Leaderboard: handle resolvido no admin-bff

`domain.LeaderboardEntry` ganhou `Handle string` (`json:"handle,omitempty"`).
`Service.GameLeaderboard` (admin-bff) chama `enrichLeaderboardHandles`
depois de `TopForGame` — reusa `handlesByID(ctx)`, o MESMO helper que já
resolve handle pra `GameSessions`/`enrichSessionHandles` (bulk-fetch de
até 1000 players do identity, best-effort: identity fora não derruba a
resposta, só o handle fica vazio).

No admin-ui, `PlayerCell` (antes privado em `SessionMatchDetail.tsx`, já
fazia exatamente esse fallback handle-ou-id-truncado pros painéis de
partida) virou exportado, com `avatarSize`/`textSize` opcionais — reusado
nas duas tabelas de `GameLeaderboards.tsx` (mini-tabela da Visão geral e
a tabela grande paginada) em vez de cada uma montar o `Group` de
avatar+id manualmente.

### 2. Player detail: id truncado + copiar

Header do `PlayerDetail.tsx`: UUID completo em `Text sm` virou
`truncate(playerId)` (helper já existente, `lib/format.ts`) em
`Text xs ff="monospace"`, com `Tooltip` mostrando o id completo (mesmo
padrão das tabelas) + um `CopyButton` do Mantine (ícone copiar/check,
`ActionIcon size="xs"`) — não existia nenhum jeito de copiar o id antes
(diferente das tabelas, onde dá pra selecionar a célula).

### 3. `EmptyState` — componente novo, compartilhado

`components/EmptyState.tsx`: ícone dimmed 28px + texto centralizado —
generaliza o hint que já existia isolado no canvas de Event Rules (spec
080, `IconHandClick` + texto). Aplicado nos 4 estados vazios de
primeiro nível (o que uma pessoa encontra ao entrar numa lista pela
1ª vez, não sub-abas aninhadas): `Rooms.tsx` (`IconDoorEnter`),
`PlayerDetail.tsx` sem jogos (`IconDeviceGamepad2`), `Games.tsx` sem
jogo registrado (`IconDeviceGamepad2`), `GameSessions.tsx` sem partida
(`IconActivity`). Estados vazios de sub-abas (PlayerAchievements,
CloudSaveView, PlayerDataView, GamePlayers, filtro sem resultado em
Games) permanecem texto simples — deliberado, pra não repetir o mesmo
ícone+texto em toda superfície secundária do console (contido, não
decorativo em todo canto).

## Critérios de Aceite

1. `go build ./services/admin-bff/...` e `tsc --noEmit` limpos.
2. Leaderboard (mini-tabela e tabela grande) mostra handle real — validado
   ao vivo (Capoeira Souls: `Ana_Riva`, `InvasorMalandro` etc. em vez de
   UUID).
3. Player detail mostra id truncado + botão de copiar funcional —
   validado ao vivo (botão presente, tooltip com id completo).
4. `EmptyState` renderiza nos 4 pontos listados — validado ao vivo
   (Rooms filtrado pra "Fechadas" sem sala nenhuma).
5. Sem erro de console em nenhuma tela tocada.

## Fora de escopo

- Resolver handle no `AchievementProgressEntry` (mesmo gap, não pedido
  nesta rodada — achado ao investigar o leaderboard, fica documentado
  pra uma próxima).
- Aplicar `EmptyState` nas sub-abas secundárias (ver "Decisões" acima —
  deliberadamente fora).
