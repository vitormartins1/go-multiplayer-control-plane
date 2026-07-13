# Spec: 027-closed-session-result-and-outcome

**Serviço:** session-service (fatias em admin-bff e admin-ui)
**Status:** Done
**Data:** 2026-07-13
**Parte de:** revisão de completude da spec 025-session-history, pedida pelo
usuário ao inspecionar o histórico real de um jogador (achado: caixinhas de
papel sumindo no `host_world_dropin`; achados adicionais na investigação: o
`Result` nunca era persistido pra NENHUM arquétipo, e `TeamByPlayer` também
não — este último trazido explicitamente pro escopo pelo usuário após a
primeira revisão crítica).

## Contexto

Auditoria do histórico persistido (spec 025) encontrou três gaps de dado,
confirmados ao vivo (o terceiro foi achado adjacente durante a própria
revisão desta spec, trazido para dentro do escopo a pedido do usuário):

1. **`Result` nunca é persistido em `closed_sessions`** (achado maior,
   afeta `queue_arena`/`battle_royale`/`coop_mission`). `closeSession` já
   tinha `sess.Result` populado em memória no momento do close (`ReportResult`
   grava `sess.Result` antes de chamar `closeSession`), mas `SaveClosed`
   nunca gravou esse campo — só participantes. Confirmado com 5 sessões
   reais fechadas com `close_reason=result`: a API devolve `"result": null`
   pra todas. Resultado: assim que a sessão sai do ring in-memory (~200) ou o
   processo reinicia, **o resultado da partida some pra sempre** — a aba
   Sessions/History cai no roster simples, sem erro visível. **Impacto maior
   do que parece**: `FfaPanel`/`PlacementPanel`/`CoopPanel` do admin-ui
   (`components/SessionMatchDetail.tsx`) derivam 100% de `s.result`
   (confirmado lendo o código: `s.result?.winLossDraw`/`?.placement`/
   `?.collective`, sem outro fallback de dado) — ou seja, **toda sessão
   fechada de `queue_arena`/`battle_royale`/`coop_mission` hoje mostra só um
   roster achatado, sem nenhum outcome/placement/score**, não é uma
   degradação parcial. Persistir `Result` desbloqueia os 3 painéis pro
   histórico sem precisar tocar no código deles.
2. **`host_world_dropin` nunca teve um "resultado" por participante**
   (achado do usuário). `ReportParticipationEvent` carrega `event`+`outcome`
   por chamada (`JOINED`/`BOSS_DEFEATED`/`HOST_KILLED`/`DIED`/`LEFT` ×
   `SUCCESS`/`FAIL`/`NEUTRAL`), publicados no fan-out, mas **nunca gravados**
   no registro do participante — `domain.Participant{PlayerID, Role,
   JoinedAt}` não tem onde guardar isso. Nem ao vivo nem no histórico dá pra
   responder "o que aconteceu com o coop-1 nesse mundo?" além de "ele
   entrou".
3. **`TeamByPlayer` (formação por time) também nunca é persistido em
   `closed_sessions`** (achado adjacente, mesma causa raiz do achado 1:
   `SaveClosed` só grava as colunas de `closed_sessions`/
   `closed_session_players`, nunca serializou `sess.TeamByPlayer`).
   Confirmado no código: `TeamByPlayer map[string]string` (domain/session.go:128)
   é imutável após o `Create` (`stampTeams`, session_service.go:326, usa essa
   mesma referência pra carimbar `TeamID` em cada entrada do `Result` no
   `ReportResult`) e a serialização pro DTO (`toSessionDTO`,
   http_handler.go:384: `dto.Teams = teamsDTO(s.TeamByPlayer)`) já sabe
   montar a grade de times a partir dele — só falta esse dado sobreviver ao
   fechamento da sessão. Efeito hoje: sessões `queue_arena` em modo times
   (2v2 etc.) perdem o agrupamento visual por time no histórico
   (`TeamCards` no admin-ui nunca ativa pra sessão fechada — cai direto em
   `FfaPanel`, tabela achatada por score; o outcome/placar por jogador
   continua correto porque `stampTeams` também grava `TeamID` dentro de cada
   entrada do `Result`, então esse dado nem se perde de vez — só a grade
   visual por time não é reconstruída).

## Comportamento

### Result persistido (queue_arena / battle_royale / coop_mission)

- WHEN uma sessão fecha com `sess.Result != nil` (fechamento por
  `ReportResult`) -> THE SYSTEM SHALL persistir o `Result` em
  `closed_sessions` junto com o resto do registro (mesma transação/chamada
  de `SaveClosed`, sem mudar a garantia best-effort já existente da spec 025).
- WHEN a sessão fecha por qualquer outro motivo (ttl/admin/abandoned/
  host_left/host_killed) -> `Result` fica ausente (`nil`) — comportamento
  inalterado, nada a reportar.
- `GET /admin/games/{gameId}/sessions?state=closed` e
  `GET /admin/players/{playerId}/sessions?gameId=` SHALL devolver `result`
  preenchido pras sessões que fecharam com resultado — o campo `result` no
  DTO **já existe** (`resultDTO`/`toResultDTO`); só falta a origem do dado
  (`domain.Session.Result`) vir preenchida quando lida do store.

### Times persistidos (queue_arena em modo times)

- WHEN uma sessão fecha com `sess.TeamByPlayer` não-vazio (modo com times,
  ex.: 2v2) -> THE SYSTEM SHALL persistir `TeamByPlayer` em `closed_sessions`
  junto com o resto do registro (mesma transação de `SaveClosed`).
- WHEN a sessão é FFA/host_world (`TeamByPlayer` nil/vazio) -> nada a
  persistir, comportamento inalterado (`teams` ausente no DTO, como hoje).
- `GET /admin/games/{gameId}/sessions?state=closed` e
  `GET /admin/players/{playerId}/sessions?gameId=` SHALL devolver `teams`
  preenchido pras sessões em modo times — o campo `teams` no DTO **já
  existe** e já sabe montar a grade a partir de `TeamByPlayer`
  (`teamsDTO`, inalterado); só falta a origem do dado vir preenchida quando
  lida do store.

### Outcome por participante (host_world_dropin)

- WHEN `ReportParticipationEvent` é aplicado com sucesso (publicado no
  fan-out) -> THE SYSTEM SHALL gravar o `event`/`outcome` recebidos como o
  **último evento conhecido** (`LastEvent`/`LastOutcome`) do participante
  correspondente — mesmo participante que `JoinWorld`/`OpenWorld` já
  registrou (por `PlayerID`), sem criar um registro novo.
- IF o participante ainda não existe na lista (não deveria ocorrer em uso
  normal — evento reportado sem `JoinWorld`/`OpenWorld` prévio) -> THE
  SYSTEM SHALL ignorar silenciosamente (no-op) — não é o papel deste evento
  criar participante.
- Re-entrada com o MESMO papel (dedupe já existente, spec 025) NÃO reseta
  `LastEvent`/`LastOutcome` — só a próxima ocorrência de evento atualiza.
  Re-entrada com papel DIFERENTE (spec 025: "registra a ocorrência mais
  recente") também reseta `LastEvent`/`LastOutcome` para vazio (mesmo
  espírito: é uma nova ocorrência de participação).
- O DTO de participante (`participantDTO`, já existe) SHALL ganhar
  `lastEvent`/`lastOutcome` (omitidos se vazios).

## Contratos de Interface

### Domínio (session-service)
```go
type Participant struct {
    PlayerID string
    Role     string
    JoinedAt time.Time
    // LastEvent/LastOutcome são o último evento de participação conhecido
    // (spec 027) — só existem em host_world_dropin (ReportParticipationEvent).
    // Vazios = nunca reportou evento além do join (JOINED ainda não chegou,
    // ou dado legado anterior a esta spec).
    LastEvent   string // "JOINED" | "BOSS_DEFEATED" | "HOST_KILLED" | "DIED" | "LEFT"
    LastOutcome string // "SUCCESS" | "FAIL" | "NEUTRAL"
}

// Session ganha (na verdade já tinha, só nunca ia pro store):
// Result *MatchOutcome — nenhuma mudança de tipo, só passa a ser persistido.
// TeamByPlayer map[string]string — idem, já existia, só passa a ser persistido.
```

### Schema (migration 003 do session-postgres)
```sql
ALTER TABLE closed_sessions ADD COLUMN result JSONB;
ALTER TABLE closed_sessions ADD COLUMN team_by_player JSONB;

ALTER TABLE closed_session_players ADD COLUMN last_event TEXT NOT NULL DEFAULT '';
ALTER TABLE closed_session_players ADD COLUMN last_outcome TEXT NOT NULL DEFAULT '';
```
`result` é o `domain.MatchOutcome` serializado via `encoding/json` padrão do
Go (nomes de campo PascalCase — é armazenamento interno do repositório, não
um contrato externo; mesmo espírito do `params_json` já usado em
`ProfileSnapshot`). `NULL` = sessão sem resultado (ttl/admin/abandoned/
host_left/host_killed, ou host_world_dropin — que nunca tem `Result`).

`team_by_player` é `map[string]string` (`domain.Session.TeamByPlayer`)
serializado do mesmo jeito (JSON cru, sem contrato externo — a forma
pública já é `teamsDTO`, que lê esse mapa e monta a grade). `NULL` = sessão
sem times (FFA, host_world) — mesmo bucket de "nada a mostrar" que `result`.

### HTTP admin (aditivo, DTOs já existem)
```json
{
  "participants": [
    {"playerId": "coop-1", "role": "coop", "joinedAt": "...",
     "lastEvent": "DIED", "lastOutcome": "NEUTRAL"}
  ],
  "result": {"kind": "win_loss_draw", "winLossDraw": [...]}
}
```

## Critérios de Aceite

1. Partida `queue_arena`/`battle_royale`/`coop_mission` fechada com
   `ReportResult` -> `result` presente no histórico persistido (validar
   restart real: fechar, reiniciar o session-service, `result` continua lá).
2. Evento `ReportParticipationEvent` (qualquer tipo) -> `lastEvent`/
   `lastOutcome` do participante atualizados, ao vivo e após persistência.
3. Sessão sem resultado (ttl/admin/abandoned/host_left) -> `result` ausente,
   sem erro.
4. Dado legado (sessões fechadas antes desta migration) -> `result: null`,
   `teams` ausente, `lastEvent`/`lastOutcome` vazios — sem quebra, mesmo
   padrão de "bucket vazio" já usado nas migrations anteriores do projeto.
5. Reentrega/reprocessamento não duplica nem corrompe `LastEvent`/
   `LastOutcome` (é um `UPDATE` idempotente por natureza — última escrita
   vence).
6. Partida `queue_arena` em modo times (2v2) fechada com `ReportResult` ->
   `teams` presente no histórico persistido, mesma grade que aparecia ao
   vivo (validar restart real, mesmo teste do critério 1); FFA/host_world
   segue sem `teams`, sem erro.

## Fora de escopo (v1)

- **Duração por participante** (`LeftAt`/tempo de permanência) — só
  `JoinedAt` existe hoje; fica pra quando houver caso real pedindo (ex.:
  "tempo médio até a morte de um invasor").
- **Um "outcome" sintético para o HOST** quando o mundo fecha por
  `HOST_KILLED` — o host não reporta evento sobre si mesmo; o `CloseReason`
  da sessão (já existe, `host_killed`/`host_left`/etc.) já comunica o
  desfecho do mundo do ponto de vista do host. Inventar um outcome próprio
  pro host é decisão de produto, não escopo desta spec técnica.
- **Resetar `LastEvent`/`LastOutcome` em re-entrada com o MESMO papel** —
  mantém o comportamento atual (sem reset); só re-entrada com papel
  diferente reseta, espelhando a regra já existente pra `Role`/`JoinedAt`.
