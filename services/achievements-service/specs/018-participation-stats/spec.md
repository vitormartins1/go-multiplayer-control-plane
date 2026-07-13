# Spec 018 — Stats no participation_event (host_world_dropin)

> Status: **implementada** (achado durante o exercício de conteúdo do jogo
> "Achievements Test" — spec-curta, aprovada por decisão direta do Vitor,
> 2026-07-10)
> Fatia: pequena/aditiva — estende um pipeline existente pro MESMO padrão já
> usado nos outros 3 arquétipos; sem decisão técnica nova (mistura idêntica
> ao que `ApplyMatchResult`/`ApplyPlacementResult`/`ApplyCollectiveResult`
> já fazem).
>
> **Correção 2026-07-13:** esta spec implementou só o lado de RECEBER `stats`
> aqui no achievements-service — o `session-service` nunca chegou a enviar o
> campo (nem no proto, nem no envelope), então na prática `Stats` sempre
> chegava `nil` por este caminho e nenhum achievement por stat de
> host_world_dropin disparava de verdade (no-op silencioso, sem erro visível).
> Achado ao especificar `analytics-service/011-raw-stat-capture`; fechado por
> `session-service/026-participation-event-stats`.

## Contexto

A spec 015 (stats-as-facts) implementou `Facts.WithStats` genericamente no
domain, mas **excluiu deliberadamente host_world_dropin** ("HWD fora — sem
stats no envelope") por falta de caso de uso na época. Testando o motor a
fundo com o jogo "Achievements Test" (perfis multiplayer novos, achievements
por stat, regras compostas com comparadores), essa exclusão virou um bloqueio
real: **nenhum achievement por stat, e nenhuma regra de evento com condição
`stat.*`, funciona hoje em host_world_dropin** — o envelope `participation_event`
não carrega stats, `Service.ApplyParticipationEvent` não os recebe, e o
repositório nunca chama `recordStatProgress` nesse caminho. Os outros 3
arquétipos (queue_arena, battle_royale, coop_mission) já suportam stats de
ponta a ponta desde a spec 015/ADR-0023 A1 — esta spec só fecha a assimetria.

## Comportamento (EARS)

1. O envelope `participation_event` (kind do consumer) DEVE aceitar um campo
   opcional `stats` (`map[string]int64`, mesmo formato dos demais kinds).
   Ausente/vazio = comportamento atual inalterado (compatibilidade total).
2. `Service.ApplyParticipationEvent` DEVE somar os stats aos facts via
   `Facts.WithStats` antes de `EvaluateRules` — regras game-scoped em
   host_world_dropin passam a poder usar condições `stat.<chave>` com
   qualquer operador (V2a), igual já funciona em battle_royale/coop_mission.
3. O repositório DEVE aplicar os stats na MESMA transação idempotente do
   evento (por `eventID`/fallback determinístico) — critério por stat
   (cumulative soma, per_match guarda o melhor valor) avança junto com o
   critério por evento, espelhando exatamente o loop de
   `ApplyMatchResult`.
4. Nenhuma mudança de contrato pros outros 3 arquétipos nem pro formato do
   envelope existente além do campo novo opcional.

## Critérios de aceite

- [x] Envelope com `stats` presente atualiza achievements por stat
      (cumulative e per_match) escopados ao jogo, exatamente como
      queue_arena já faz.
- [x] Envelope sem `stats` (ou `{}`) continua funcionando idêntico a antes —
      zero regressão nos testes existentes de participation.
- [x] Regra de evento game-scoped com condição `stat.*` (comparador) casa
      contra os facts do envelope HWD.
- [x] Testes de domain/service/repository/consumer atualizados; `-race`
      verde no achievements-service e no monorepo.
- [x] E2E via SQS/LocalStack: envelope HWD com `stats` desbloqueia um
      achievement por stat E uma regra composta (ex.: Flawless Victory,
      `stat.deaths lte 0`); envelope sem `stats` não quebra o fluxo de
      evento puro (ex.: `world_joined`).

## Fora de escopo

- Qualquer mudança nos outros 3 arquétipos (já suportam stats).
- Validação/whitelist de quais stats um modo host_world_dropin pode declarar
  (esse é outro gap já registrado em memória — `HostWorldDropInParams` não
  tem `ScoringStats`, diferente dos outros 3 — cosmético, não bloqueia esta
  spec; fica pra decisão separada).
