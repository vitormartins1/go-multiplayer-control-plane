# Changelog do contrato control plane ↔ jogo

Tudo que muda o que o **jogo** (cliente e Dedicated Server) vê do control plane: os `.proto`
(`session.v1`, `matchmaking.v1`), as rotas REST de jogador, os tokens (JWT do jogador e connect token) e os
contratos por arquétipo em [`docs/game-models/`](../game-models/README.md). Mudança interna entre serviços não
entra aqui.

Formato [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/), versões [SemVer](https://semver.org/lang/pt-BR/):

- **MAJOR** — quebra: campo/RPC removido ou com significado novo, erro novo onde antes havia sucesso, claim de token
  obrigatório novo que o jogo precisa mandar. Pacote proto novo (`v2`) quando for quebra de wire.
- **MINOR** — aditivo: campo ou RPC novo e opcional, comportamento novo que o jogo pode ignorar.
- **PATCH** — correção sem mudança de contrato.

Regras de proto (valem desde já): nunca reutilizar número de campo (use `reserved`); dentro de `v1` só mudança
aditiva; o `.pb.go` sempre regenerado com `buf generate` (nunca editado à mão).

## [Não lançado]

### Security
- `POST /sessions` (login por senha): 5 falhas para o mesmo handle em 15 min bloqueiam o handle até a janela
  vencer, com `429 too_many_attempts` (spec `identity/007`). Pela regra deste arquivo "erro novo" seria MAJOR, mas só
  atinge quem já errou a senha 5 vezes seguidas e o login do jogo em produção é o da Steam, que não passa por aqui —
  fica registrado como correção de segurança. O cliente deve tratar 429 como "tente mais tarde".

## [1.5.0] — 2026-09-27

### Added
- `session.v1.Session/Heartbeat` (spec `session/032`): o jogo do host (bearer ou `x-world-token`) ou o Dedicated
  Server avisa que o mundo segue vivo e recebe `interval_seconds`. Depois do primeiro heartbeat o mundo não fecha
  mais aos `SESSION_TTL` (30 min) — só se ficar `WORLD_HEARTBEAT_TTL` (90 s) sem aviso, com motivo `abandoned`.
  Quem não manda heartbeat continua com a regra antiga.

## [1.4.0] — 2026-09-27

### Added
- Topologia `p2p` no `host_world_dropin` (spec `session/031`, ADR-0032): o jogo do host hospeda o mundo, sem
  Dedicated Server. Campos novos, todos opcionais para quem segue em dedicado:
  `session.v1.OpenWorldRequest.host_connection` (7), `OpenWorldResponse.host_connection` (4) e `.world_token` (5),
  `JoinWorldResponse.host_connection` (3), `matchmaking.v1.InvadeResponse.host_connection` (4) e
  `SummonUpdate.host_connection` (6).
- Metadata `x-world-token` (host de mundo p2p) aceita em `ReportParticipationEvent`, `GetSessionConfig`,
  `LeaveWorld` e `CloseWorld`, só para o próprio mundo.
- O perfil aceita `topology: "p2p"` em `host_world_dropin` (antes: 422 em qualquer arquétipo). Os demais arquétipos
  continuam só `dedicated`.
- Em p2p, o claim `server` do connect token carrega o `host_connection`.

## [1.3.0] — 2026-09-26

### Added
- `POST /sessions/steam` no identity (spec `identity/005-steam-login`): o cliente manda o ticket de
  `ISteamUser::GetAuthTicketForWebApi("<STEAM_TICKET_IDENTITY>")` em hex (`{ticket, gameId?}`) e recebe o mesmo JWT
  do login por senha (claim extra `idp: "steam"`). Na primeira vez a conta é criada e vinculada ao SteamID64. Erros:
  401 `invalid_ticket`, 403 `banned`, 503 `provider_unavailable`, 404 `provider_not_supported` (login Steam não
  configurado no deploy). Nada muda para quem já usa login por senha ou anônimo.

## [1.2.0] — 2026-09-26

### Added
- Autenticação no gRPC do session (spec `session/030`, ADR-0030): metadata `authorization: Bearer <JWT>` do
  jogador (`OpenWorld`, `LeaveWorld`, `CloseWorld`) e `x-game-server-key` do Dedicated Server (`ReportResult`,
  `ReportParticipationEvent`, `GetSessionConfig`, `LeaveWorld`, `CloseWorld`). Padrão `warn`: sem credencial
  continua funcionando (só é contado). **Virar para `enforce` será MAJOR (2.0.0)** e só acontece depois que o jogo
  mandar as credenciais.
- `OpenWorld` aceita `host_player_id` vazio quando o bearer vem: o host é o `sub` do token.
- `session.v1.JoinWorldRequest.summoner_player_id` (campo 4, interno ao control plane).

### Changed
- `SummonSign` só invoca sinal no mundo de quem é o host (`PermissionDenied` para qualquer outro) — antes qualquer
  jogador autenticado invocava sinais no mundo de outro.

## [1.1.0] — 2026-09-26

### Added
- `session.v1.ListInvadableWorldsRequest.mode_id` (campo 2, opcional): filtra o diretório pelo modo; vazio = todos
  os modos (spec `matchmaking/012`).
- Convenção para identidade do boss em `BOSS_DEFEATED`: `stats: {"boss_<id>": 1}` (decisão D3; contrato
  `host_world_dropin`).

### Changed
- `matchmaking.v1.Invade` passa a encontrar só mundos abertos no **mesmo modo** do invasor (vazio = `default`). O
  jogo deve mandar o mesmo `mode_id` do `OpenWorld` (spec `matchmaking/012`, decisão D2).
- JWT do jogador (identity) passa a trazer `iss = go-multiplayer-control-plane/identity` e `aud = game-backend`, e
  os serviços exigem os dois. Sem impacto para o jogo, que só lê o `sub`; tokens emitidos antes do deploy (15 min de
  validade) precisam de novo login.

### Fixed
- `ReportParticipationEvent`: o reenvio de um `HOST_KILLED` cujo publish falhou era respondido com `NotFound` e o
  evento se perdia; agora a mesma ocorrência (mesmo `event_id`) é republicada (spec `session/029`).
- `ReportParticipationEvent`/`ReportResult` com o fan-out travado respondem `Internal` em ~5s (antes ficavam
  pendurados até o deadline do cliente). `Internal` = reenviar a mesma ocorrência.

## [1.0.0] — 2026-07-13

Linha de base: o contrato exercitado de ponta a ponta pelo Folclore Souls (Fases 0–5 do data plane) e pelo Packet
Arena — `session.v1` (CreateSession, ReportResult, OpenWorld, JoinWorld, LeaveWorld, CloseWorld,
ListInvadableWorlds, ReportParticipationEvent com `event_id` e `stats`, GetSessionConfig), `matchmaking.v1`
(FindMatch, Invade, PostSummonSign, ListSigns, SummonSign), connect token EdDSA com claims `role`/`team_id`/
`mode_id` validado via JWKS do session, e os contratos por arquétipo em `docs/game-models/`.
