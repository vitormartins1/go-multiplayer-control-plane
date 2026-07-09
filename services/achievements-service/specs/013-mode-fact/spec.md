# Spec 013 — Fact `mode` nas event rules (F1 do adendo ADR-0028)

**Status:** Approved (fatia pequena/aditiva — spec curta, sem plan/tasks)
**Serviço:** achievements-service (consumer + domain + service); toque mínimo
no admin-ui (sugestões no canvas)
**Relaciona:** ADR-0028 (adendo — dimensão de facts), ADR-0027 (motor),
ADR-0024 (`modeId` aditivo nos envelopes)

## Contexto

Todos os envelopes do fan-out já carregam `modeId` (ADR-0024) e o consumer do
achievements o ignorava. Nos arquétipos de 1 fact (coop/queue) as seeds
esgotam o espaço de regras; expor o modo como fact destrava regras por modo
("vitória no ranked") com **zero mudança de contrato**.

## Comportamento

- O consumer decodifica `modeId` nos 4 kinds e o repassa ao service.
- As normalizadoras ganham o fact **`mode`** via decorator cross-kind
  (`Facts.WithMode`): presente **só quando o envelope traz `modeId`** —
  envelope antigo/sem modo não ganha a chave (regra com `mode=` simplesmente
  não casa; nunca casa com valor vazio).
- Seeds/globais intocadas (nenhuma referencia `mode` — subset match ignora
  chaves extras). Zero regressão por construção.
- Admin-ui (canvas): o form de criação sugere a chave `mode` e, para ela, os
  `modeId`s cadastrados no arquétipo selecionado.

## Critérios de aceite

1. Envelope com `modeId=X`: regra do jogo `{mode: X, ...}` casa; sem `modeId`,
   não casa (e nenhuma chave `mode` vazia aparece nos facts).
2. Regras globais existentes continuam casando exatamente como antes
   (testes de regressão dos 4 kinds passam sem alteração de expectativa).
3. `go test ./... -race` verde no serviço; build do admin-ui verde.
