# ADR-0027: Regras de mapeamento evento→achievement-event-type genéricas e data-driven

**Status:** Accepted
**Data:** 2026-07-09
**Serviço(s) afetado(s):** achievements-service, admin-bff, admin-ui
**Relaciona:** ADR-0023 (perfil de multiplayer por jogo — outcome polimórfico por arquétipo), ADR-0002 (arquitetura hexagonal), spec `achievements-service/008-stat-criteria` (critério por métrica, mesmo espírito), spec `achievements-service/009-participation-consumer` (host_world_dropin), `021-participation-event-id`

---

## Contexto

O achievements-service traduzia o resultado bruto de uma partida/participação em **event types de gameplay** (o vocabulário que um achievement casa via `event_type`) através de **4 funções Go hardcoded**, uma por arquétipo (ADR-0023):

- `ParticipationEventTypes(role, event, outcome)` — host_world_dropin (switch por combinação papel×evento×outcome).
- `PlacementEventTypes(placement)` — battle_royale (played/win/top_n por colocação).
- `CollectiveEventTypes(outcome)` — coop_mission (played/success/failed por desfecho coletivo).
- Lógica inline em `Service.ApplyMatchResult` — queue_arena (win_loss_draw).

Cada arquétipo novo (ou cada jogo querendo um event type **próprio** pra uma combinação já existente — ex.: "raid_defeated" em vez de "invasion_won" pra um jogo específico) exigia **mexer em código Go e reimplantar o serviço**. O admin-ui espelhava esse vocabulário num dicionário TypeScript hardcoded (`EVENT_VOCABULARY`, só usado como *sugestão*/validação de drift, nunca fonte de verdade) que precisava ser **atualizado manualmente** toda vez que um sink Go ganhava um event type novo — os dois lados podiam (e às vezes ficavam) dessincronizados.

Isso contradiz o princípio já estabelecido pelo ADR-0023 (§ "novos jogos entram por cadastro, não por código") — que resolveu esse mesmo problema para `scoringStats` (métricas), mas nunca foi estendido ao vocabulário de **eventos**.

## Alternativas Consideradas

### Opção A: Status quo — 4 funções hardcoded, uma por arquétipo
- **Prós:** zero trabalho; já testado; simples de ler pra quem conhece o arquétipo.
- **Contras:** duplicação estrutural (4 funções quase idênticas); um jogo não pode ter vocabulário próprio pra uma combinação já existente; front precisa espelhar manualmente. Descartada — é o problema que este ADR resolve.

### Opção B: Motor de regras genérico e data-driven (escolhida)
- **Prós:** UMA função avaliadora (`EvaluateRules`) substitui as 4; a regra em si vira dado (tabela `event_rules`), editável via admin sem redeploy; suporta **override por jogo** (um jogo pode somar um event type próprio a uma combinação, sem afetar os demais); front lê o mesmo dado (elimina o dicionário hardcoded/drift).
- **Contras:** indireção nova (regra não é mais "vai direto no código-fonte"); precisa de uma camada de normalização por `kind` de envelope (host_world_dropin/battle_royale/coop_mission/queue_arena têm formas de resultado diferentes, ADR-0023 outcome polimórfico) pra produzir um mapa de "facts" comum.

### Opção C: Motor de regras totalmente genérico (comparadores, ranges, operadores lógicos)
- **Prós:** cobriria casos como "colocação <= N" sem hardcode do limiar.
- **Contras:** over-engineering — nenhuma evidência hoje pede operadores além de igualdade exata; o único caso com limiar numérico (`battle_royale`'s top-N) é resolvido normalizando um fact derivado (`tier=top_n`) na camada fina de conversão, sem precisar de um operador `<=` no motor. Descartada por ora — mesmo princípio do ADR-0023 (evitar DSL sem evidência).

### Opção D: Mover o vocabulário pro perfil de multiplayer (game-registry), não pro achievements-service
- **Prós:** ficaria ao lado de `scoringStats`, no mesmo lugar conceitual.
- **Contras:** o achievements-service já é quem *consome* e *aplica* o vocabulário (é o dono do conceito "event type que desbloqueia conquista"); colocar no game-registry criaria uma dependência síncrona nova entre os dois serviços (o achievements-service nunca chamou o game-registry) só pra buscar dado que só ele usa. Descartada — mantém o acoplamento em zero, igual era antes.

## Decisão

Adotamos a **Opção B**. Nova tabela `event_rules` (achievements-service, migration 008):

```
id, game_id (nullable — NULL = regra global do arquétipo), archetype,
match (JSONB, subconjunto exato de facts), event_types (TEXT[])
```

- **Regra global** (`game_id IS NULL`): vale pra **todo jogo** que usa aquele arquétipo — semeada uma vez por migration, replicando exatamente o comportamento das 4 funções antigas (zero regressão).
- **Override do jogo** (`game_id` setado): um jogo específico pode **somar** event types próprios a uma combinação de facts, sem redeploy. União, não substituição — os globais continuam valendo.
- **Avaliador único** `domain.EvaluateRules(rules, gameID, facts)`: casa toda regra cujo `Match` é subconjunto exato de `facts` (regra vazia `{}` sempre bate — o caso "played" incondicional) e une os `EventTypes` das que baterem, sem duplicar.
- **Normalizadoras finas** por `kind` de envelope (`ParticipationFacts`, `PlacementFacts`, `CollectiveFacts`, `WinLossDrawFacts`) — só conversão estrutural (o outcome polimórfico do ADR-0023 vira um `map[string]string` comum), **sem** regra de negócio; a regra em si mora inteiramente no dado.
- **Limitação intencional:** o limiar numérico do battle_royale (`top_n <= 10`) continua hardcoded na normalização — não é vocabulário de evento, é parâmetro do arquétipo; adotar um operador de comparação genérico fica pra quando houver evidência de necessidade (mesmo princípio da Opção C descartada).
- **Admin:** novos endpoints `GET/POST/DELETE /admin/games/{id}/event-rules` (achievements-service) + proxy no admin-bff — só regras **do jogo** são editáveis por aqui; globais são somente leitura (protegidas contra remoção acidental). Nova aba "Event Rules" no admin-ui (CRUD completo), e a aba Achievements passa a ler o vocabulário conhecido desta mesma fonte (elimina o dicionário `EVENT_VOCABULARY` hardcoded no front, que existia só como sugestão/validação de drift).

## Consequências

### Positivas
- Elimina a duplicação estrutural das 4 funções Go — um único avaliador testável isoladamente.
- Novo jogo com vocabulário próprio (ex.: renomear "invasion_won" pra "raid_defeated" só pro jogo X) não exige mais mexer em Go nem redeploy.
- Front e backend compartilham a mesma fonte de verdade — acaba o risco de vocabulário dessincronizado entre TSX e Go.
- Mesma filosofia do ADR-0023 (`scoringStats`) agora também vale pro lado de eventos — consistência arquitetural.

### Negativas / Trade-offs
- Uma indireção a mais: entender "por que este evento vira aquele achievement" agora exige olhar uma tabela, não só o código-fonte.
- `Service.ApplyParticipationEvent`/`ApplyPlacementResult`/`ApplyCollectiveResult`/`ApplyMatchResult` ganham uma consulta a mais (`RulesForArchetype`) antes da transação existente — custo desprezível na escala atual (uma query indexada por `archetype`), mas é uma dependência nova que não existia.
- A tabela cresce com N arquétipos × M regras — ainda pequena (13 linhas no seed), mas overrides por jogo podem crescer sem limite hoje (sem paginação no endpoint admin — aceitável na escala atual, revisar se um jogo cadastrar centenas de regras).

### Neutras / Implicações
- As constantes de nome de evento (`EventInvasionWon` etc., em `participation.go`/`placement.go`/`collective.go`) permanecem como **vocabulário de referência** (usadas no seed da migration e em achievements pré-cadastrados), mas as FUNÇÕES de mapeamento foram removidas — só a nomenclatura ficou.
- Reabre a pergunta (não respondida aqui) de se o admin deveria poder **desabilitar** uma regra global pra um jogo específico (hoje só dá pra somar, nunca suprimir) — fica pra evidência futura, mesmo padrão do ADR-0023.

## Referências
- ADR-0023 (perfil de multiplayer, outcome polimórfico por arquétipo — o pré-requisito conceitual desta decisão).
- `services/achievements-service/migrations/008_event_rules.sql`, `internal/domain/event_rule.go`.
- `services/admin-ui/src/pages/GameEventRules.tsx` (CRUD novo) e `GameAchievements.tsx` (badge de tipo + vocabulário via API).
