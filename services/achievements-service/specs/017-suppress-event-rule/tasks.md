# Tasks 017 — Suprimir regra global por jogo (V2b)

- [x] T1. Migration 010 (`event_rule_suppressions`) + mount no compose +
      aplicar manualmente no banco em uso.
- [x] T2. Domain: `EventRule.Suppressed`; guarda em `EvaluateRules`;
      `Suppress`/`Unsuppress` nas Ports (`EventRuleService` +
      `EventRuleRepository`); teste do avaliador ignorando `Suppressed`.
- [x] T3. Repository: `RulesForArchetype` exclui suprimidas (`NOT EXISTS`);
      `ListEffective` computa `suppressed` (`LEFT JOIN`); `Suppress`
      (existe+global → idempotente) e `Unsuppress` (existe supressão →
      idempotente-erro); testes de integração dos 4 casos (suprimir regra
      global, suprimir regra de jogo → 404, suprimir 2x → ok, restaurar sem
      supressão ativa → 404).
- [x] T4. Service: `SuppressEventRule`/`UnsuppressEventRule` (passthrough) +
      testes com mock do repo.
- [x] T5. Transport: rotas `POST`/`DELETE .../event-rules/{id}/suppress`,
      `eventRuleDTO.Suppressed`; testes de handler (204/404).
- [x] T6. Admin-bff: passthrough nas 4 camadas (domain/service/client/http) +
      testes espelhando os de `UpdateEventRule`.
- [x] T7. `go test ./... -race` verde no achievements-service e no admin-bff.
- [x] T8. E2E via SQS/LocalStack: dois jogos de teste do mesmo arquétipo;
      suprimir uma regra global só no jogo A, publicar envelope que bateria
      nela para os dois jogos, confirmar que só B recebe o event
      type/achievement progride; restaurar em A, confirmar que volta a
      bater; dados de teste removidos.
- [x] T9. Commit + rebuild dos containers (achievements-service + admin-bff)
      pro admin-ui (spec 051) poder consumir os endpoints novos.
