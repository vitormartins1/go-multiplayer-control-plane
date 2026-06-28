# ADR-0015: Estratégia de persistência — database-per-service + polyglot com Postgres default

**Status:** Accepted
**Data:** 2026-06-28
**Serviço(s) afetado(s):** todos, infra
**Relaciona:** ADR-0005 (mesma lógica diferida, para compute)

---

## Contexto

O ADR-0005 definiu *como* escolher **compute** por serviço, mas trata **banco de dados** só de passagem (linha 38: "gerenciado vs auto-provisionado, mesma lógica diferida"). Com o identity pronto e o **segundo** serviço (achievements) nascendo, surgem duas perguntas que ainda não têm política registrada e que, se respondidas ad hoc, acoplam os serviços ou viram polyglot sem critério:

1. **Isolamento:** cada serviço tem seu banco, ou compartilham um?
2. **Tecnologia:** todos usam Postgres, ou cada domínio escolhe a sua?

A decisão precisa existir **antes** de o achievements modelar seus dados, porque define se serviços podem se relacionar por JOIN (acoplados) ou só por id/API (independentes).

## Alternativas Consideradas

### Opção A: Database-per-service + polyglot com Postgres default (decisão diferida por serviço)
- **Prós:** cada serviço é dono dos seus dados (deploy/escala/tecnologia independentes); comparação fiel com os pares AWS da tese (cada serviço Go ↔ seu equivalente gerenciado); um default forte (Postgres) evita explosão de tecnologias.
- **Contras:** mais instâncias/peças operacionais; sem JOIN cross-service (composição vira responsabilidade da aplicação/API); polyglot exige expertise em mais de um banco.

### Opção B: Shared database (um banco, tabelas/schemas por serviço)
- **Prós:** uma instância só; JOINs fáceis entre domínios; menos operação.
- **Contras:** acoplamento de schema — uma migração de um serviço pode quebrar outro; perde deploy/escala independentes; é o anti-padrão clássico de microsserviços.

### Opção C: Padronizar uma única tecnologia para todos
- **Prós:** uniformidade operacional máxima.
- **Contras:** subótimo por domínio (ranking real-time em SQL, blob de save em linha de tabela, eventos em volume num relacional); não exercita a comparação polyglot da tese.

## Decisão

Adotamos **database-per-service** com **polyglot persistence disciplinada**:

1. **Propriedade:** cada serviço é dono exclusivo do seu schema. **Nenhum serviço acessa o banco de outro** — integração só por **API ou evento**.
2. **Relacionamento:** FK/JOIN **apenas dentro** de um serviço. Entre serviços, relaciona-se por **id + API/evento** (ex.: achievements guarda `player_id`, nunca faz JOIN em `accounts`). Compartilhar a **instância** é uma decisão de custo aceitável; compartilhar **tabelas**, não.
3. **Tecnologia:** **Postgres é o default**. Desvia-se só quando o *padrão de acesso do domínio* claramente pede outra coisa, e cada desvio é **justificado no `plan.md`** (ou em ADR próprio se for grande), espelhando o par AWS da tese:

   | Padrão de acesso | Banco local | Par AWS |
   |---|---|---|
   | CRUD relacional / KV (identity, achievements, player-data) | Postgres (+ JSONB) | DynamoDB / RDS |
   | Blob grande (cloud-save) | MinIO/S3 | S3 |
   | Ranking real-time (leaderboard) | Redis (sorted sets) | ElastiCache / DynamoDB |
   | Eventos em volume (analytics) | colunar/time-series | Kinesis / Athena |

4. **Multi-tenant:** "vários jogos" é uma **dimensão (`game_id`) dentro do serviço**, não um banco por jogo.
5. **Isolamento físico vs lógico** (container/instância dedicada vs database separado no mesmo servidor) é **diferido ao `plan.md`/infra** de cada serviço, por custo — desde que o isolamento **lógico** (sem tabelas compartilhadas) seja sempre respeitado.

Esta é uma **meta-decisão**, como o ADR-0005: define *como* escolher o banco e adia a escolha vinculante de cada serviço para o `plan.md`.

## Consequências

### Positivas
- Serviços independentes (deploy, escala, evolução de schema); comparação 1:1 com os pares AWS; polyglot com critério, não espaguete.

### Negativas / Trade-offs
- Mais instâncias e padrões operacionais; composição de dados entre domínios passa a ser trabalho da aplicação (sem JOIN); mais de uma tecnologia para dominar.

### Neutras / Implicações
- O `plan.md` de cada serviço declara o banco escolhido e justifica desvios do Postgres.
- Localmente, pode-se compartilhar uma instância Postgres com **databases separados** por serviço se o custo pedir, mantendo o isolamento lógico.
- achievements-service: **Postgres** (par AWS = DynamoDB), **container próprio**, sem `game_id` no MVP (evolução conhecida).

## Referências
- ADR-0005 (seleção de compute diferida — mesma filosofia)
- ADR-0008 (compute genérico vs serviços-de-jogo AWS)
- ADR-0009 (decomposição fiel ao GameKit — cada feature um stack próprio)
- docs/ROADMAP.md (pares Go ↔ AWS por serviço)
