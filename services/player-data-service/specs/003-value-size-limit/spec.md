# Spec: 003-value-size-limit

**Serviço:** player-data-service
**Status:** Done
**Data:** 2026-09-26
**Nível:** fatia pequena (limite de entrada no handler; sem mudança de schema)
**Origem:** revisão de produto 2026-09 (`docs/review/REVISAO-2026-09.md`, lacuna G5)

## Contexto

`PUT /me/data/{namespace}/{key}` lia o corpo inteiro (`io.ReadAll`) sem teto e gravava no Postgres (JSONB). Qualquer
jogador autenticado podia gravar valores arbitrariamente grandes — custo de banco e de memória sem limite. O
equivalente AWS da tese (GameKit User Gameplay Data, sobre DynamoDB) tem teto de **400 KB por item**.

## Comportamento

- WHEN o corpo do `PUT /me/data/{namespace}/{key}` passa de **400 KiB** (409.600 bytes) -> THE SYSTEM SHALL
  responder **413** `value_too_large` sem gravar nada.
- Corpo até o limite segue o comportamento atual (JSON válido → 204; inválido → 400 `invalid_value`).

## Critérios de aceite

1. Corpo de 400 KiB + 1 byte → 413 `value_too_large`; o service não é chamado.
2. Corpo JSON válido de exatamente 400 KiB → 204.
3. README do serviço documenta o limite.

## Fora de escopo

Quota total por jogador/jogo (FEATURE-MAP) — só o teto por valor.
