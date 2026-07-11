# Spec 011 — ScoringStats no host_world_dropin

> Status: **Done** (achado registrado durante o estresse-teste do
> motor de achievements/event rules — jogo "Achievements Test" — corrigido a
> pedido direto do Vitor, 2026-07-10)
> Fatia: pequena/aditiva — replica um campo que os outros 3 arquétipos já
> têm; sem decisão técnica nova.

## Contexto

`HostWorldDropInParams` era o único dos 4 arquétipos sem o campo
`ScoringStats` (`queue_arena`, `battle_royale` e `coop_mission` já tinham).
Consequência prática: o canvas de event rules sempre marcava qualquer stat
usada por uma regra/achievement de host_world_dropin como "drift" (nunca
declarável), mesmo depois da spec achievements/018 destravar stats de
verdade nesse arquétipo. Puramente um gap de declaração/UI — os stats já
funcionavam funcionalmente sem o campo (nada valida contra a lista hoje),
mas o campo existe pra dar visibilidade/documentação de quais métricas o
data plane deve reportar, igual os outros 3 já fazem.

## Comportamento (EARS)

1. `HostWorldDropInParams` (domain) DEVE ganhar `ScoringStats []string`,
   mesmo formato/semântica dos outros 3 arquétipos.
2. `validateHostWorldDropIn` DEVE validar cada item com o mesmo regex
   (`scoringStatRE`) já usado pelos outros arquétipos.
3. Persistência (JSONB `multiplayer_profile`) e os dois DTOs HTTP
   (repository + transport) DEVEM incluir `scoringStats` no round-trip,
   preservando o mecanismo de chave extra (`ExtraParams`/follow-up #14) —
   `scoringStats` entra no vocabulário TIPADO (`hostWorldDropInKnownKeys`),
   não pode vazar pra `ExtraParams`.
4. O admin-ui DEVE expor um campo de edição (`TagsInput`, mesmo padrão dos
   outros arquétipos) no formulário do modo host_world_dropin.

## Critérios de aceite

- [x] `scoringStats` sobrevive ao round-trip completo (domain → JSONB →
      domain) sem vazar pra `ExtraParams`.
- [x] `scoringStats` sobrevive ao round-trip do DTO HTTP
      (`PUT`/`GET /games/{id}/multiplayer-profile`).
- [x] Item inválido (`"Kills"`, maiúscula) rejeitado na validação, igual aos
      outros 3 arquétipos.
- [x] Admin-ui: formulário do modo host_world_dropin tem o mesmo campo de
      stats que os outros arquétipos; `modeStats()` (canvas) já lia o campo
      de forma genérica — passa a funcionar sem mudança de lógica ali.
- [x] `-race` verde (unit + integração) no game-registry; build verde no
      admin-ui.

## Fora de escopo

- Qualquer validação cruzada entre `scoringStats` declaradas e os stats
  realmente reportados no envelope (isso já é o "drift" do canvas, que
  passa a funcionar corretamente pra host_world_dropin como consequência
  desta spec, sem lógica nova).
