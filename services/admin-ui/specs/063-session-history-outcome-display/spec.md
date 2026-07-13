# Spec: 063-session-history-outcome-display

**Serviço:** admin-ui
**Status:** Done
**Data:** 2026-07-13
**Parte de:** session-service/027-closed-session-result-and-outcome +
admin-bff/025-session-history-outcome-passthrough

## Contexto

Achado do usuário ao revisar sessões reais de "Capoeira Souls"
(`host_world_dropin`): sessões FECHADAS mostram só a caixinha do host,
listando co-op/invasores num roster achatado "OUTROS PARTICIPANTES" — apesar
dessas caixinhas aparecerem corretamente durante a partida ao vivo.

Causa raiz (`components/SessionMatchDetail.tsx:482-566`, `WorldRoleCards`):
o componente monta os cards de papel a partir de `s.roleSlots` — um campo
**só populado ao vivo**, a partir do mapa em memória do session-service
(`RoleSlot{Role,Capacity,Occupied,Occupants}`). Sessões lidas do histórico
persistido (`ClosedByGame`/`ClosedByPlayer`, spec 025) nunca tiveram
`roleSlots` no payload — só `participants[]` (`{playerId, handle, role,
joinedAt}`). Sem slots, `slots.length === 0`, `assigned` só contém o host, e
TODO mundo exceto o host cai no fallback `others` → lista achatada.

Ao mesmo tempo (session-service/027 + admin-bff/025), `participants[]`
ganha `lastEvent`/`lastOutcome` por jogador — hoje não há lugar nenhum na UI
que mostre "o que aconteceu" com um participante de `host_world_dropin"
fechado, nem ao vivo nem no histórico (segundo achado do usuário).

## Comportamento

### Cards de papel a partir de `participants[]` (fallback quando `roleSlots` vazio)

- WHEN `s.roleSlots` está vazio/ausente E `s.participants` tem ao menos um
  registro com `role` preenchido (sessão `host_world_dropin` fechada) -> THE
  SYSTEM SHALL agrupar `participants[]` por `role` e desenhar os mesmos
  cards visuais já usados no modo ao vivo (`ROLE_META` pro rótulo/cor,
  mesmo componente `PlayerCell`) — sem capacity/occupied (não existe mais
  esse dado numa sessão fechada; o card mostra só quem ocupou, sem "vagas
  livres" nem contagem X/Y).
- O card do HOST continua vindo de `s.hostPlayerId` (já correto hoje,
  inalterado). **Confirmado no código do session-service** (`OpenWorld`,
  session_service.go:712): o host É gravado em `Participants` com
  `Role: "host"` desde a abertura do mundo — logo `participants[]` SEMPRE
  vai conter o host com `role: "host"`. O agrupamento por papel SHALL
  excluir explicitamente `role === "host"` (já coberto pelo card dedicado),
  senão duplica o host num card extra de papel "host" (`ROLE_META.host` já
  existe e teria cor/rótulo próprios — o bug de duplicação seria silencioso,
  não um crash).
- WHEN `s.roleSlots` tem dados (sessão ao vivo) -> comportamento ATUAL
  inalterado (usa slots, com capacity/occupied/vagas livres).
- WHEN nem `roleSlots` nem `participants[].role` têm dado (sessão legada
  pré-spec-025, ou arquétipo sem papéis) -> mantém o fallback achatado atual
  (`RosterBlock` com `others`), sem mudança.

### Badge de outcome por participante

- Cada `PlayerCell` desenhado dentro de um card de papel (tanto no modo ao
  vivo recém-adaptado — se `lastEvent` vier preenchido via polling — quanto
  no histórico) SHALL exibir um badge pequeno com `lastOutcome` quando
  presente: `SUCCESS` = verde, `FAIL` = vermelho, `NEUTRAL` = cinza. Ausente
  -> nenhum badge (não inventar "desconhecido").
- Tooltip do badge (reaproveita o padrão `Tooltip` já usado pro id) mostra
  `lastEvent` por extenso (ex.: "BOSS_DEFEATED" -> pode ficar o valor cru,
  sem dicionário de tradução nesta fatia — fora de escopo criar um mapa de
  labels amigáveis pros valores de evento).

### Aba History do player

- `PlayerHistory.tsx` (spec 061) já lista sessões fechadas por jogador
  com colunas Quando/Modo/Papel/Motivo/Duração/Participantes. Papel do
  próprio jogador (coluna "Papel") já vem de `participants[]` filtrado pelo
  `playerId` da página — SEM MUDANÇA estrutural. Só ganha, na mesma coluna
  ou logo ao lado, o outcome do jogador nessa sessão (mesmo badge de cor
  usado nos cards), quando `lastOutcome` estiver presente.

## Contratos de Interface

```ts
// lib/api.ts — SessionParticipant ganha os 2 campos novos (passthrough do
// admin-bff/025, que já os repassa 1:1 do session-service/027)
export interface SessionParticipant {
  playerId: string;
  handle: string;
  role?: string;
  joinedAt: string;
  lastEvent?: string;
  lastOutcome?: "SUCCESS" | "FAIL" | "NEUTRAL";
}
```

Novo helper em `SessionMatchDetail.tsx` (ou módulo próximo): agrupar
`participants[]` por `role` em `Map<string, SessionParticipant[]>`,
reaproveitando `ROLE_META` já existente pra rótulo/cor — mesmo padrão visual
dos cards ao vivo, sem introduzir um novo sistema de card.

## Critérios de Aceite

1. Sessão `host_world_dropin` FECHADA com participantes co-op/invasor
   (dado real de "Capoeira Souls") -> aba Sessions mostra os cards por
   papel (não só host + lista achatada).
2. Mesma sessão, participante com `lastOutcome: "FAIL"` -> badge vermelho
   visível no card correspondente.
3. Sessão ao vivo (roleSlots presente) -> comportamento visual idêntico ao
   atual, sem regressão (capacity/occupied/vagas livres continuam
   aparecendo).
4. Sessão fechada de outro arquétipo (`queue_arena`/`battle_royale`/
   `coop_mission`, sem `role` em `participants[]`) -> sem mudança visual
   (esses arquétipos já usam `TeamCards`/`FfaPanel`/etc., não
   `WorldRoleCards` — confirmado que `MatchPanel` já roteia certo por
   `archetype`/`s.teams.length` antes de mexer).
4a. Sessão `queue_arena` em modo times (2v2) FECHADA -> `TeamCards` renderiza
    a grade por time (não cai mais em `FfaPanel`), uma vez que o
    session-service (spec 027) e o admin-bff (spec 025) passem a entregar
    `teams` preenchido — sem qualquer mudança de código neste componente,
    só passa a RECEBER o dado que faltava (achado adjacente, trazido pro
    escopo da correção).
5. Aba History do player -> outcome visível quando presente, sem quebrar o
   layout quando ausente (sessões de outros arquétipos, ou dado legado).

## Fora de escopo

- Dicionário de tradução de `lastEvent`/valores de evento pra texto amigável
  em pt-BR (ex.: "BOSS_DEFEATED" -> "Derrotou o chefe") — mostrar o valor
  cru é aceitável nesta fatia; vira melhoria de copy se o usuário achar
  confuso na prática.
- Trazer `capacity`/vagas-livres pro card de sessão fechada — esse dado não
  existe mais depois que o mundo fecha (não fazia parte da spec 025/027
  persistir isso); mostrar só quem participou é a informação real
  disponível.
- Mudança em `TeamCards`/`FfaPanel`/`PlacementPanel`/`CoopPanel` (outros
  arquétipos) — já leem `s.result` corretamente; passam a RECEBER dado
  quando session-service/027 persistir `Result`, sem qualquer mudança de
  código nesta fatia.
