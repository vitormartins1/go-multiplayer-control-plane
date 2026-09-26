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
