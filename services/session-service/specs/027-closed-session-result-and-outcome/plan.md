# Plan — 027 resultado e outcome no histórico

## Decisões técnicas

1. **`result` como JSONB, serializado com `encoding/json` padrão do Go
   (sem struct tags de JSON)** — o repositório é dono do formato de
   armazenamento interno; não precisa espelhar o camelCase da API pública
   (isso já é feito por `resultDTO`/`toResultDTO` na camada de transporte,
   inalterada). Evita acoplar `domain.MatchOutcome` a tags JSON só por causa
   de storage. Precedente no projeto: `ProfileSnapshot.ParamsJSON []byte`
   guarda JSON cru sem re-tipar.
2. **`LastEvent`/`LastOutcome` como campos simples no `Participant`, não uma
   lista de histórico de eventos** — a spec 025 já decidiu que "timeline de
   eventos dentro da sessão" é fora de escopo (viraria spec própria em cima
   do envelope existente, que já flui pro fan-out/analytics). Aqui só
   guardamos o ÚLTIMO evento conhecido, o suficiente pra responder "o que
   aconteceu com esse participante" sem duplicar o papel do fan-out.
3. **Atualização de `LastEvent`/`LastOutcome` com um lock próprio, dentro de
   `ReportParticipationEvent`, logo após `countParticipation` e ANTES do
   switch por tipo de evento** — **correção em cima da primeira leitura do
   código**: ao contrário do que a v1 deste plano assumia, não há lock
   aberto nesse ponto (o primeiro `s.mu.Lock()` já foi liberado logo depois
   de ler `gameID`/`modeID`, antes do publish). É preciso um `s.mu.Lock()`
   dedicado, com re-lookup de `sess` (mesmo padrão idempotente que o case
   `Died`/`Left` já usa pra `removeFromWorld` — sessão pode ter fechado
   entre o publish e aqui). Confirmado que `removeFromWorld` só mexe em
   `RoleSlots[role].Occupants` (libera a vaga), NUNCA em `sess.Participants`
   — então a atualização de outcome é segura mesmo quando o evento também
   dispara remoção do slot.
4. **`result` nullable, sem valor default** — `NULL` cobre tanto "sessão sem
   resultado" quanto "dado legado" (mesmo bucket, sem distinção — não há como
   diferenciar retroativamente, e não precisa: as duas situações têm a mesma
   ação na UI, "sem resultado a mostrar").
5. **Sem mudança na Port `HistoryStore`** — a assinatura de `SaveClosed(ctx,
   sess *domain.Session)` já recebe `sess.Result`/`sess.TeamByPlayer`; só o
   adapter Postgres precisa aprender a serializar. `ClosedByGame`/
   `ClosedByPlayer` idem, só aprendem a desserializar.
6. **`TeamByPlayer` serializado do mesmo jeito que `Result`** (JSONB,
   `encoding/json` puro, `nil`-safe) — achado adjacente trazido pro escopo
   (spec.md, achado 3): `stampTeams` já usa essa referência pra carimbar
   `TeamID` em cada entrada do `Result`, e `teamsDTO(s.TeamByPlayer)`
   (http_handler.go:384) já sabe montar a grade — só falta o dado
   sobreviver ao fechamento. Mesmo tratamento de nil/vazio que `Result`
   (FFA/host_world não têm `TeamByPlayer`, fica `NULL`).

## Arquivos por camada

**session-service:**
- `migrations/003_closed_session_result_and_outcome.sql` (novo) — `ALTER
  TABLE` nas duas tabelas (schema do spec.md), incluindo `team_by_player`.
- `internal/domain/session.go` — `Participant.{LastEvent,LastOutcome}
  string` (novo). `MatchOutcome` e afins NÃO mudam (só passam a ser
  persistidos, não alteram forma).
- `internal/service/session_service.go`:
  - `ReportParticipationEvent`: novo helper `setParticipantOutcome`
    (participants []domain.Participant, playerID, event, outcome string) —
    percorre e atualiza in-place; chamado sob o lock já existente, logo após
    `s.countParticipation(evt.Event)` e antes do `switch evt.Event`.
  - `upsertParticipant` (JoinWorld): ao trocar de papel numa re-entrada,
    zera `LastEvent`/`LastOutcome` junto com o reset de `Role`/`JoinedAt`
    (mesmo bloco condicional `if p.Role != role`).
- `internal/repository/postgres_history.go`:
  - `SaveClosed`: serializa `sess.Result` e `sess.TeamByPlayer` via
    `json.Marshal` (nil-safe: `nil` vira `NULL` na coluna); INSERT em
    `closed_session_players` ganha `last_event`/`last_outcome`.
  - `scanClosedSessions`: SELECT ganha `result` e `team_by_player` (scan pra
    `[]byte`/nullable, depois `json.Unmarshal` se não-nulo), atribuindo em
    `domain.Session.Result`/`domain.Session.TeamByPlayer`.
  - `attachParticipants`: SELECT ganha `last_event`/`last_outcome`, scan pros
    campos novos do `domain.Participant`.
- `internal/transport/http_handler.go`:
  - `participantDTO` ganha `LastEvent`/`LastOutcome string` com
    `json:"lastEvent,omitempty"`/`json:"lastOutcome,omitempty"`.
  - `participantsDTO()` (helper de conversão) propaga os campos novos.
  - `toResultDTO`/`resultDTO`/`teamsDTO` — SEM MUDANÇA (já existem, só
    passam a receber dado não-nil quando a sessão vem do store com
    resultado/times).
- Testes: `session_service_test.go`/`history_test.go` (LastEvent/Outcome
  atualizados por evento; reset em troca de papel), `postgres_history_test.go`
  (round-trip de `result` + `team_by_player` + `last_event`/`last_outcome`,
  incluindo `NULL`), `http_handler_test.go` (DTO expõe os campos novos).

**admin-bff:** spec-irmã `025-session-history-outcome-passthrough` — só
passthrough, tipos já espelham o shape existente (`MatchResult`,
`SessionParticipant`, `SessionTeam`/`Teams` já existentes desde a spec 018 —
nenhuma mudança de tipo necessária, só volta a chegar dado não-vazio).

**admin-ui:** spec-irmã `063-session-history-outcome-display` — conserta
`WorldRoleCards` pra desenhar cards de papel a partir de `participants[]`
quando `roleSlots` vier vazio (sessão fechada), com badge de outcome;
`FfaPanel`/`PlacementPanel`/`CoopPanel`/`TeamCards` já sabem renderizar
`result`/`teams` — não precisam de mudança, só vão passar a RECEBER o dado
(`TeamCards` volta a ativar sozinho pra sessões fechadas em modo times, via
o mesmo roteamento por `s.teams.length > 0` que `MatchPanel` já usa hoje pro
caminho ao vivo).

## Ordem de implementação

Migration → domínio (`Participant`) → service (`setParticipantOutcome` +
reset em `upsertParticipant`) → repository (serialização `result` +
`team_by_player` + `last_event`/`last_outcome`) → transport (DTO) → testes
→ build/test completo → validação ao vivo (reabrir uma sessão
`host_world_dropin` real, reportar eventos, fechar, conferir `lastEvent`/
`lastOutcome` persistidos; fechar uma partida `queue_arena`/`battle_royale`/
`coop_mission` real, restart do session-service, conferir `result`
sobrevivendo; fechar uma partida `queue_arena` em modo times, conferir
`teams` sobrevivendo e `TeamCards` renderizando no histórico).
