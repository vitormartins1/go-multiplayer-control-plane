# Plan 015 — Stats como facts

## Decisões técnicas

1. **`Facts.WithStats` no domain** — decorator cross-kind (padrão do
   `WithMode`, spec 013): `stat.<chave>` = `strconv.FormatInt(v, 10)`. As
   normalizadoras seguem estruturais; a composição fica no service.
2. **Coop avalia POR JOGADOR sempre** (não só quando há stats): facts
   idênticos → resultado idêntico, e o código fica uniforme com os outros
   kinds (sem branch "tem stat?"). N por sessão é pequeno (teamSize ≤ ~4);
   simplicidade > micro-otimização. Zero mudança de output pra regras sem
   `stat.*`.
3. **Placement usa o mapa já com `survived_ms` injetado** (o mesmo
   `withSurvivedMs` do critério por métrica) — `stat.survived_ms` funciona
   nas regras e os dois consumidores do mapa ficam consistentes.
4. **Consumer intocado** — os stats já chegam decodificados nos structs de
   domínio; só o service muda a composição dos facts.
5. **Admin-ui**: `keySuggestions` soma `stat.<s>` das stats do arquétipo
   (reusa o `statsForArch` da spec 045 — declaradas + em uso).

## Arquivos

| Arquivo | Ação |
|---|---|
| `internal/domain/event_rule.go` | `FactStatPrefix` + `WithStats` |
| `internal/domain/event_rule_test.go` | WithStats + flawless victory |
| `internal/service/achievement_service.go` | compõe WithStats nos 3 kinds; coop por jogador |
| `internal/service/achievement_service_test.go` | coop por jogador (MVP) |
| admin-ui `pages/GameEventRulesCanvas.tsx` | sugestões `stat.*` |

## Riscos / atenção

- `WithStats` muta o map (igual WithMode) — sempre compor sobre Facts recém
  construídos pela normalizadora (nunca reusar entre jogadores).
- E2E flawless via SQS (win+deaths:0 casa; win+deaths:2 e loss+deaths:0 não).
