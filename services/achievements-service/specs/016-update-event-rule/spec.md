# Spec 016 — Atualizar event rule do jogo (PUT)

> Status: **implementada** (aprovada em 2026-07-09)
> Fatia: pequena/aditiva — espelha o create existente (mesma validação, mesmo
> DTO); sem decisão técnica nova além de PUT-substitui-tudo.

## Contexto

A edição in-canvas de regra (spec admin-ui/050) precisa persistir mudanças, e
hoje o serviço só tem GET/POST/DELETE. Sem update, "editar" na UI seria
delete+create — não atômico e trocaria o `id` da regra (quebraria o nó
selecionado no canvas e qualquer referência futura).

## Comportamento (EARS)

1. **Quando** receber `PUT /admin/games/{gameId}/event-rules/{id}` com body
   `{match, eventTypes}`, o serviço DEVE substituir integralmente o match e
   os eventTypes da regra `{id}` **escopada ao jogo** `{gameId}`, preservando
   `id` e `archetype`, e responder `200` com o DTO da regra atualizada.
2. **Se** a regra for global, de outro jogo ou inexistente, DEVE responder
   `404` (mesma proteção do DELETE — global nunca é alcançável por jogo).
3. A validação DEVE ser a mesma da criação: `eventTypes` não vazio e
   minúsculo, condições do match válidas (`MatchCond.Valid()`); inválido →
   `400`.
4. `archetype` NÃO é editável (mudar arquétipo = outra regra; remove e cria).
5. Rota admin protegida pelo mesmo verifier das demais.

## BFF (passthrough)

- `PUT /api/games/{gameId}/event-rules/{id}` no admin-bff, repassando o body
  opaco (`json.RawMessage` no match — mesmo formato do create pós-V2a).

## Critérios de aceite

- [x] PUT em regra do jogo altera match/eventTypes e mantém id (200).
- [x] PUT em regra global ou id de outro jogo → 404; body inválido → 400
      (inclui archetype no payload — decodeStrict rejeita).
- [x] Testes de service+transport espelhando os do create; `-race` verde
      (294 no total).
- [x] E2E: regra `stat.objectives ≥ 100` editada pra `≥ 40` via PUT no BFF;
      envelope coletivo com objectives=50 via SQS destravou o achievement
      de teste (só casa na versão editada); PUT em global → 404; dados de
      teste removidos.
