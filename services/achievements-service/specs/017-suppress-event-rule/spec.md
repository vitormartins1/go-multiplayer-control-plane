# Spec 017 — Suprimir regra global por jogo (V2b)

> Status: **implementada** (aprovada e validada e2e em 2026-07-10)
> Fatia: substancial — nova tabela, novo conceito de domínio, cruza serviço
> (achievements-service + admin-ui via spec `admin-ui/051`).
> Origem: pergunta aberta do ADR-0027 ("reabre a pergunta... se o admin
> deveria poder desabilitar uma regra global pra um jogo específico"),
> penúltima etapa do roadmap V2 do ADR-0028 (V2a comparadores → **V2b
> supressão** → V2c prioridade, gated por evidência).

## Contexto

Hoje um jogo só pode **somar** event types a uma combinação de facts já
coberta por uma regra global (override, `game_id` setado — ADR-0027). Não
existe caminho para um jogo **desligar** uma regra global para si: as globais
são imutáveis e sempre avaliadas para todo jogo do arquétipo. Isso é uma
limitação real — ex.: um jogo do arquétipo `battle_royale` que não quer emitir
`battle_royale_top_n` (porque não tem achievement pra isso e considera o
vocabulário confuso) não tem como remover essa regra da própria avaliação sem
afetar os demais jogos do arquétipo.

## Modelo de dados

Nova tabela `event_rule_suppressions` (migration 010 — a 009 foi
`match_operators`), modelando a relação jogo→regra-global-suprimida:

```sql
CREATE TABLE event_rule_suppressions (
    game_id    TEXT NOT NULL,
    rule_id    TEXT NOT NULL REFERENCES event_rules(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (game_id, rule_id)
);
```

**Por que tabela própria, não coluna em `event_rules`:** a alternativa óbvia
seria um array `suppressed_by_games TEXT[]` na própria linha da regra global.
Descartada — quebraria o invariante "regra global é imutável/somente-leitura"
de forma literal (a linha compartilhada por N jogos passaria a ser escrita
toda vez que UM jogo suprime/restaura, com risco de corrida entre jogos
diferentes mexendo na mesma linha) e não modela naturalmente uma relação N:M
auditável. Uma tabela dedicada segue o mesmo padrão arquitetural já usado no
serviço (relação como tabela própria, não coluna sobrecarregada), mantém as
linhas de `event_rules` intocadas (o "somente-leitura" continua verdade em
todos os sentidos) e a supressão é trivialmente reversível (`DELETE` da
linha = restaurar). `ON DELETE CASCADE` é higiene (globais não são removíveis
pela API hoje, mas a constraint não deve sobreviver a uma regra apagada por
outro caminho, ex. migration futura).

Suprimir uma regra de **jogo** (não-global) não faz sentido — override já é
removível via `DeleteEventRule`. A supressão só existe para `game_id IS NULL`.

## Domínio

- `EventRule` ganha `Suppressed bool` — populado só por `ListEffective`
  (leitura do admin); regras devolvidas por `RulesForArchetype` (caminho de
  avaliação) já vêm com as globais suprimidas EXCLUÍDAS pelo repositório,
  então esse campo nunca precisa ser lido ali.
- `EvaluateRules` ganha uma guarda defensiva (`if r.Suppressed { continue }`)
  — barata, e evita que um uso futuro indevido (ex.: alguém passar o
  resultado de `ListEffective` direto pro avaliador, em vez de
  `RulesForArchetype`) reative uma regra suprimida silenciosamente. Função
  continua pura/testável.
- `EventRuleService` (Port) ganha:
  ```go
  // SuppressEventRule desliga uma regra GLOBAL para o jogo gameID — a linha
  // da regra não é tocada; só deixa de contribuir event types nas avaliações
  // DESTE jogo (outros jogos do arquétipo continuam recebendo normalmente).
  // Idempotente. ErrEventRuleNotFound se ruleID não existir ou não for
  // global (override de jogo não é suprimível — é removível via Delete).
  SuppressEventRule(ctx context.Context, gameID, ruleID string) error

  // UnsuppressEventRule restaura uma regra global suprimida (volta a valer
  // para gameID). ErrEventRuleNotFound se não havia supressão ativa para o
  // par (gameID, ruleID) — mesma semântica de "não existe" do Delete.
  UnsuppressEventRule(ctx context.Context, gameID, ruleID string) error
  ```
- `EventRuleRepository` (Port) ganha `Suppress`/`Unsuppress` espelhando a
  mesma semântica (idempotente / `ErrEventRuleNotFound`).

## Comportamento (EARS)

1. **Quando** receber `POST /admin/games/{gameId}/event-rules/{id}/suppress`,
   o serviço DEVE gravar a supressão `(gameId, id)` e responder `204`. Se a
   regra `{id}` não existir ou não for global, DEVE responder `404`. Suprimir
   uma regra já suprimida DEVE ser um no-op (idempotente, `204`).
2. **Quando** receber `DELETE .../event-rules/{id}/suppress`, o serviço DEVE
   remover a supressão e responder `204`. Se não havia supressão ativa para o
   par, DEVE responder `404`.
3. **Quando** avaliar um envelope do jogo `gameId` (qualquer arquétipo), as
   regras globais suprimidas por `gameId` NÃO DEVEM contribuir event types —
   nem sozinhas, nem em combinação com outras regras (a exclusão acontece
   antes do `EvaluateRules`, no `RulesForArchetype`).
4. **Quando** o admin listar as regras efetivas do jogo
   (`GET .../event-rules`), cada regra global suprimida por este jogo DEVE
   vir com `suppressed: true`; as demais (globais não suprimidas e todas as
   de jogo) DEVEM vir com `suppressed: false`.
5. A supressão é **por jogo** — suprimir para o jogo A nunca afeta a
   avaliação nem a listagem do jogo B.
6. Rotas admin protegidas pelo mesmo verifier das demais.

## HTTP / BFF

- achievements-service: `POST` e `DELETE`
  `/admin/games/{gameId}/event-rules/{id}/suppress` (sem body).
- admin-bff: proxy 1:1 em `/api/games/{gameId}/event-rules/{id}/suppress`
  (mesmo padrão de passthrough das rotas de event-rules existentes).
- `eventRuleDTO` ganha `Suppressed bool \`json:"suppressed"\`` (sempre
  presente; `false` para regras de jogo, onde o conceito não se aplica).

## Critérios de aceite

- [x] `POST .../suppress` numa regra global exclui-a das avaliações do jogo
      (envelope que antes emitia o event type dela deixa de emitir, só para
      este jogo); outro jogo do mesmo arquétipo continua recebendo
      normalmente (regressão-zero para os demais).
- [x] `POST .../suppress` numa regra de jogo (override) ou id inexistente →
      `404`. Suprimir duas vezes seguidas → `204` nas duas (idempotente).
- [x] `DELETE .../suppress` sem supressão ativa → `404`; com supressão ativa,
      remove e a regra volta a contribuir na próxima avaliação.
- [x] `GET .../event-rules` reflete `suppressed: true/false` corretamente por
      jogo (dois jogos distintos, um suprime, o outro não — listagens
      divergem só nesse campo).
- [x] Testes de domain (`EvaluateRules` ignora `Suppressed`), service e
      transport (espelhando os de create/update/delete); `-race` verde.
- [x] E2E: suprimir uma regra global de `battle_royale` pro jogo de teste,
      publicar envelope via SQS/LocalStack que antes emitia o event type
      suprimido, confirmar que não emite mais (achievement associado não
      progride); restaurar, confirmar que volta a emitir; dados de teste
      removidos.

## Fora de escopo

- V2c (prioridade/exclusividade first-match-wins) — gated por evidência,
  não faz parte desta fatia.
- Qualquer UI — coberta pela spec `admin-ui/051`.
