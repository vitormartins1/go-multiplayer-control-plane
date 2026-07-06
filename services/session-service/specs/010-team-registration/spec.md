# Spec: 010-team-registration

**Serviço:** session-service
**Status:** Aprovada (design confirmado 2026-07-06)
**Data:** 2026-07-06
**ADR:** 0023 (perfil de multiplayer por jogo) — **fatia "composição de times"** (session é o dono canônico dos times)

## Contexto

A spec `009-polymorphic-result` tornou o resultado polimórfico: o `ReportResult` recebe um `MatchOutcome` (`win_loss_draw` com `PlayerOutcome{ TeamID, Outcome, Score, Stats }`) e o fan-out (ADR-0020) o propaga. O campo `TeamID` já existe — mas hoje **só o game server o preencheria**, e nada garante que ele seja o rótulo canônico usado na formação da partida.

A contraparte no matchmaking (spec `matchmaking/005-team-composition`) passa a **atribuir os jogadores a lados** na formação de um match `teams`. Esta spec faz o session-service **receber e registrar** essa composição na criação da sessão e se tornar o **dono canônico** dos times: no `ReportResult`, o session **carimba o `TeamID`** de cada jogador a partir da composição registrada, em vez de confiar no que o server reporta.

Por quê o session é o **dono canônico** do `team_id`: o rótulo do time é uma decisão do control plane (quem formou o match), não do gameplay. Duas consequências:

1. **Para rodar o gameplay NvN**, o Dedicated Server (repo `go-game-dataplane`, Unity) **precisa conhecer** os times — spawn points, cores, friendly-fire, placar por lado. O session propaga o time ao data plane pelo **connect token**: cada token passa a carregar o `team_id` do jogador (claim assinado Ed25519, lido no *connection approval* do NGO — o mesmo ponto onde o server já valida o token, ADR-0010/0011). Sem novo canal, sem round-trip.
2. **Para o resultado**, o session **carimba** o `team_id` a partir da composição registrada, em vez de confiar no que o server ecoar. Isso garante um `team_id` confiável para o fan-out → leaderboard/achievements/analytics por time (fatias seguintes), materializando o "granularidade por time é derivada pelos consumidores, não hardcoded" do ADR-0023.

**Dependência cross-repo (documentada, não bloqueante para o control plane):** o data plane do queue_arena precisará (a) **ler o `team_id` do connect token** para o gameplay NvN e (b) já migrar o `ReportResult` do formato antigo `PlayerResult{won,score}` para o `win_loss_draw`/`PlayerOutcome` da spec `009` (dívida pré-existente). O contrato completo control↔data plane vive em `docs/game-models/queue_arena-dataplane-contract.md`.

## Comportamento

### Registrar a composição de times na criação da sessão

- WHEN o matchmaking chama `CreateSession` com uma **composição de times** (jogador→time) → THE SYSTEM SHALL registrar a sessão guardando, além dos jogadores, o **time de cada jogador**.
- WHEN a criação é `ffa` (sem composição de times / times vazios) → THE SYSTEM SHALL registrar a sessão **sem times** (`TeamID` vazio por jogador) — comportamento equivalente ao atual; nenhuma regressão para jogos FFA.
- THE SYSTEM SHALL emitir **um connect token por jogador** e, quando a sessão tem times, **incluir o `team_id` do jogador como claim** no token (vazio/ausente em FFA). A alocação do server e o restante do grant são inalterados; o data plane lê o `team_id` no *connection approval* para o gameplay NvN.
- IF a composição for internamente inconsistente (um jogador em dois times; jogador nos times mas ausente da lista de players; ou vice-versa) → THE SYSTEM SHALL rejeitar com `InvalidArgument` (**ErrInvalidTeams**). O matchmaking é a origem confiável, mas o session valida na fronteira (defesa em profundidade).

### Carimbar o `TeamID` no resultado a partir da composição registrada

- WHEN o game server chama `ReportResult` com o `win_loss_draw` outcome → THE SYSTEM SHALL, para cada `PlayerOutcome`, **substituir** o `TeamID` pelo time **registrado** daquele jogador na sessão (a autoridade é a composição da formação, não o que o server enviou). Em `ffa`, o `TeamID` permanece vazio.
- IF um `PlayerOutcome` referencia um jogador que **não** pertence à sessão → THE SYSTEM SHALL manter o comportamento de validação de outcome já existente (spec `009`); o carimbo de time se aplica só aos jogadores conhecidos da sessão.
- THE SYSTEM SHALL propagar no fan-out (envelope, ADR-0020) o `TeamID` **canônico** assim resolvido, de modo que os consumidores (leaderboard/achievements/analytics) recebam times confiáveis sem conhecer a formação.
- THE SYSTEM SHALL manter o restante do `ReportResult` inalterado: liberar o server, encerrar a sessão (`CloseResult`), idempotência e validações de outcome da spec `009`.

### Compatibilidade

- Sessões `ffa` e jogos sem times seguem funcionando sem mudança observável.
- O envelope de fan-out permanece o **v2** da spec `009` (o `team_id` já faz parte dele); esta fatia só garante que ele venha **preenchido corretamente** para `teams`. Nenhuma nova versão de envelope.

## Contratos de Interface

### Protobuf (`session/v1/session.proto`)

- `CreateSessionRequest` ganha a **composição de times**. Duas formas possíveis (decisão do plan):
  - **(a)** `repeated Team teams = 4;` com `Team{ string team_id = 1; repeated string players = 2; }`, mantendo `repeated string players = 2` como a lista completa (retrocompat: `ffa` manda `players` e omite `teams`); **ou**
  - **(b)** substituir a lista plana por `repeated PlayerSlot players` com `PlayerSlot{ string player_id; string team_id; }`.
  - **Preferência:** (a) — aditiva, `teams` opcional, zero quebra para os chamadores FFA e para o formato atual.
- `CreateSessionResponse`, `ConnectGrant`, `ReportResultRequest`/`WinLossDrawResult`/`PlayerOutcome` — **inalterados**. O `PlayerOutcome.team_id` continua no proto (o server pode mandar, mas o session o sobrescreve).

### Ports (domínio)

- **`SessionService.Create`** passa a receber a composição de times:
  - de `Create(ctx, matchID, gameID string, players []string)` para algo como `Create(ctx, matchID, gameID string, teams []TeamAssignment)` com `TeamAssignment{ TeamID string; Players []string }` (ou `players []string` + `map[playerID]teamID`). Forma exata no plan; `ffa` = um único assignment sem `TeamID`.
- **`Session`** (agregado) passa a guardar o **time por jogador** (ex.: `Teams []TeamAssignment` ou `Players []Participant{ PlayerID; TeamID }`). A escolha (manter `Players []string` + mapa paralelo vs. trocar por `[]Participant`) é decisão do plan.
- **`ConnectTokenIssuer.Issue`** ganha o `teamID`: de `Issue(playerID, matchID, serverAddr)` para `Issue(playerID, matchID, serverAddr, teamID)`, emitindo o claim `team_id` (omitido/vazio em FFA). JWKS inalterado.
- **`ServerAllocator`**, **`ResultPublisher`**, **`ServerRegistry`** — **inalterados**.

### Erros de domínio

- **Novo** `ErrInvalidTeams` — composição inconsistente na criação (jogador em dois times, players/teams divergentes). Mapeado no transport para `InvalidArgument`.

## Modelo de Dados

O session persiste sessão? No MVP a sessão é **em memória** (spec 001/008: `s.sessions` map). A composição de times entra **na mesma estrutura em memória** da sessão (sem migration). Se/quando a sessão for persistida, o time por jogador acompanha — fora de escopo aqui.

## Fora de escopo

- **Win/loss por time no data plane** (o server decide o vencedor por time) — o server já reporta o outcome por jogador; a agregação por time nos consumidores é fatia seguinte.
- **Leaderboard/achievements/analytics por `teamID`** — consumidores; esta fatia só garante o `teamID` canônico no envelope.
- **Persistência da sessão** (hoje é em memória).
- **Implementação Unity do NvN** (ler o `team_id` do token, spawns/placar por time) — cross-repo, documentada em `docs/game-models/queue_arena-dataplane-contract.md`; aqui só se emite o claim.

## Critérios de Aceite

- [ ] `CreateSession` com `teams` registra o time de cada jogador; `ffa`/sem times não regride.
- [ ] Connect token de um jogador em `teams` carrega o claim `team_id`; em `ffa` o claim é ausente/vazio.
- [ ] `ReportResult` carimba o `TeamID` de cada `PlayerOutcome` a partir da composição registrada (ignora o que o server mandou); `ffa` mantém `TeamID` vazio.
- [ ] Composição inconsistente na criação → `ErrInvalidTeams` (`InvalidArgument`).
- [ ] O envelope de fan-out sai com `team_id` canônico para `teams` e vazio para `ffa`.
- [ ] Idempotência, liberação do server e encerramento da sessão da spec `009` preservados.
- [ ] Testes unitários (criação com times, carimbo no ReportResult, ffa sem regressão, composição inválida) com `-race`; cobertura `service/`/`domain/` ≥ 80%.
