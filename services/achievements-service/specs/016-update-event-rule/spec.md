# Spec 016 — Atualizar event rule do jogo (PUT)

> Status: **draft** (aguardando aprovação)
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

- [ ] PUT em regra do jogo altera match/eventTypes e mantém id (200).
- [ ] PUT em regra global ou id de outro jogo → 404; body inválido → 400.
- [ ] Testes de service+transport espelhando os do create; `-race` verde.
- [ ] E2E: editar regra via API e confirmar que o consumer avalia a versão
      nova (SQS/LocalStack).
