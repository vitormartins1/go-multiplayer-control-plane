# Spec 018 — Detalhe rico da sessão no admin (params do modo + resultado persistido)

**Serviço:** session-service (+ passthrough admin-bff, render admin-ui)
**Status:** Done (2026-07-08) (direção aprovada pelo usuário em 2026-07-08; origem: spec 009 do data plane `unity-queue-arena-data-plane`)
**Fatia:** aditiva ponta a ponta (campos novos + render); decisão técnica única: persistir o outcome na sessão fechada.

## Contexto

O expand da sessão na aba Sessions (e no board Rooms) mostra hoje só ciclo de vida (players, server, tempos, motivo). O session-service **já tem em mãos e descarta** as duas informações mais relevantes da partida:

1. o **snapshot do perfil do modo** (ADR-0024, spec 017) — `archetype`, `params_json` (scoreTarget, structure, teamCount…), `profile_hash`;
2. o **resultado** que passa pelo `ReportResult` (spec 009/015/016) — outcome por player com `team_id` **carimbado** (canônico), score e stats — hoje só vai pro fan-out.

Com isso o operador responde no console: "que regras valeram nesta partida?", "quem ganhou, com que placar/stats, em que time?" — sem `redis-cli` nem grepar log de pod (que morre no `Shutdown`).

## Comportamento

### session-service (dono do dado)

- `domain.Session` SHALL ganhar `Result *MatchOutcome`, preenchido no `ReportResult` **após o publish bem-sucedido** e antes do `closeSession` — o que fica gravado é exatamente o que foi publicado (com `team_id` já carimbado). Sessões fechadas por `ttl`/`admin`/`abandoned`/`host_left`/`host_killed` ficam com `Result` nil (não houve resultado).
- O DTO de `GET /admin/games/{gameId}/sessions`, `GET /admin/sessions` e `POST /admin/sessions/{matchId}/close` SHALL ganhar (todos `omitempty`, aditivos):
  - `archetype` e `profileHash` — do snapshot;
  - `modeParams` — o `params_json` **cru** do snapshot (objeto JSON; o consumidor lê as chaves que conhece, mesmo princípio do manifest);
  - `teams` — a formação registrada (`[{teamId, players[]}]`, ordenada por teamId), omitida em FFA/host_world (TeamByPlayer vazio);
  - `result` — espelho do `MatchOutcome` (tagged union): `{kind, winLossDraw?{players[{playerId,teamId,outcome,score,stats}]}, placement?{entries[…]}, collective?{outcome, players[…]}}`. Presente só quando a sessão fechou por `result`.
- Sem endpoint novo: os campos entram na listagem existente (o histórico é em memória e pequeno — `historyLimit`; um endpoint de detalhe é otimização prematura).

### admin-bff (passthrough)

- `domain.GameSession` e o client do session SHALL repassar os campos novos 1:1 (`modeParams` como JSON cru). Sem enriquecimento extra: os `playerId` do resultado se resolvem no front com os handles que o BFF já enriquece em `players`.

### admin-ui (render no expand)

- O painel expandido da sessão (aba Sessions do jogo e board Rooms, se aplicável) SHALL mostrar, quando presentes:
  - **Arquétipo** e **profile hash** junto do modo;
  - **Params do modo**: chips chave:valor genéricos a partir do `modeParams` (qualquer arquétipo; sem hardcode de chaves);
  - **Formação**: players agrupados por time (com handle/avatar já disponíveis), quando `teams` presente;
  - **Resultado**: por kind — `win_loss_draw`: tabela player · time · outcome (badge win/loss/draw) · score · stats; `placement`: tabela colocação · player · sobrevivência; `collective`: desfecho da sessão + stats por player. Ausente → nada (sessões antigas/sem result).

## Critérios de aceite

- [x] `ReportResult` grava `Result` na sessão fechada; publish falho → nada gravado, sessão segue aberta (comportamento atual preservado)
- [x] Fechamentos sem resultado (ttl/admin/abandoned/host_left/host_killed) → `Result` nil e JSON sem `result`
- [x] DTO admin com `archetype`/`profileHash`/`modeParams`/`teams`/`result` (aditivo; sessões sem snapshot seguem válidas)
- [x] BFF repassa os campos; front-check (`npm run build`/lint) verde
- [x] Testes Go (service + transport + client do BFF) verdes com `-race`; e2e manual: partida `teams-2v2` do data plane → expand mostra params (scoreTarget 5, teams 2×2), formação e resultado com times carimbados
- [x] Compat: data plane não muda (nada de contrato gRPC); consumidores antigos do JSON admin ignoram os campos novos

## Fora do escopo

- Live view / streaming de estado da partida (descartado pelo usuário em 2026-07-08)
- Persistência durável do histórico (segue em memória, `historyLimit`)
- Enriquecimento de handles dentro de `result` no BFF (join no front)
- Duração/timeline de eventos além do que já existe
