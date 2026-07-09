# Tasks 014 — Comparadores no match (V2a)

- [x] T1. Domain: `MatchCond` (+`Eq`, ops, JSON custom leniente), `condMatches`,
      `matchSubset` por operador; remove `tier`/`BattleRoyaleTopNThreshold`.
- [x] T2. Service: `validateEventRule` estrito para conds (op/tipo/lista).
- [x] T3. Repository: warning de cond inválida no scan. Transport: req/DTO com
      `MatchCond`.
- [x] T4. Migration 009 + mount no compose + aplicar no banco em uso.
- [x] T5. Testes: novos (ops, round-trip, regra top_n) + adaptação dos literais;
      `go test ./... -race` verde.
- [x] T6. Admin-ui: `matchCond.ts` (tipos + label + build), api.ts, rulesGraph
      (nó por condição), form com select de op, render tabela/painéis; build.
- [x] T7. E2E via SQS (placement 10 casa / 11 não) + rebuild dos containers.
