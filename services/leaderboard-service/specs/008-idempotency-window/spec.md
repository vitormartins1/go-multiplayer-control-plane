# Spec: 008-idempotency-window

**Serviço:** leaderboard-service
**Status:** Done
**Data:** 2026-09-26
**Nível:** fatia pequena, com uma decisão técnica registrada abaixo (D1)
**Origem:** revisão de produto 2026-09 (`docs/review/REVISAO-2026-09.md`, lacuna G8)

## Contexto

A guarda de idempotência dos boards cumulativos (spec 004) é um único SET por jogo
(`<prefix>:processed:<gameId>`) com todo `matchId` já aplicado. Membros de SET não expiram: o SET cresce para
sempre (um membro por partida, por jogo), embora a reentrega at-least-once do SQS só aconteça dentro da retenção da
fila (4 dias no default do Terraform local; máx. 14 dias na AWS; a DLQ pode redirecionar depois disso).

## Decisão

- **D1 — uma key por partida com TTL de 30 dias** (`<prefix>:processed:<gameId>:<matchId>`, `SET NX EX`), em vez do
  SET único. 30 dias cobre a retenção máxima da fila (14) + a da DLQ (14) com folga. Alternativas descartadas:
  SET por dia com `EXPIRE` (checagem em N keys por partida) e sorted set com poda por score (script mais longo, poda
  periódica) — ambas mais complexas para o mesmo efeito.
- **Compat:** o SET legado continua sendo **consultado** (`SISMEMBER`), nunca mais escrito — uma reentrega de uma
  partida aplicada antes do deploy segue detectada. Depois de 30 dias do deploy, o SET legado pode ser apagado à mão.

## Comportamento

- WHEN uma partida é aplicada pela primeira vez -> THE SYSTEM SHALL gravar a key da partida com TTL de 30 dias e
  somar os incrementos, no mesmo script atômico (inalterado: SET NX + ZINCRBY sem interleaving).
- WHEN a mesma partida chega de novo dentro da janela, ou já consta no SET legado -> no-op, `dup = true`.

## Critérios de aceite

1. Os testes de idempotência existentes continuam verdes (integração, Redis real).
2. A key de uma partida aplicada tem TTL > 0 e ≤ 30 dias.
3. Partida presente só no SET legado é tratada como dup e não soma.
