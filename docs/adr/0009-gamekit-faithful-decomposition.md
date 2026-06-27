# ADR-0009: Decomposição fiel ao GameKit (serviços separados, identity como fundação)

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** identity-service, achievements-service, player-data-service, cloud-save-service

---

## Contexto

Os equivalentes das features do AWS GameKit (Identity & Auth, Achievements, User Gameplay Data, Game State Saving) precisam de uma estratégia de decomposição. Pesquisa sobre o GameKit mostrou que a AWS provisiona **cada feature como um stack próprio** (API Gateway + Lambda + DynamoDB), com a **Identity como fundação** da qual as demais dependem para autenticar requests por jogador.

## Alternativas Consideradas

### Opção A: Serviços separados, fiel ao GameKit
- **Prós:** isolamento real; deploy/escala independentes; comparação 1:1 fiel ao GameKit; exercita auth entre serviços (realismo enterprise); cada serviço pequeno e didático.
- **Contras:** mais scaffolds; complexidade de auth entre serviços (validar JWT da identity).

### Opção B: Monólito modular (um serviço, features como módulos)
- **Prós:** mais simples; auth in-process; uma stack.
- **Contras:** uma unidade de escala/deploy; menos fiel ao GameKit; exercita menos microsserviços.

### Opção C: Híbrido por coesão (identity isolado + player-state agrupado)
- **Prós:** meio-termo.
- **Contras:** agrupamento arbitrário; perde escala independente entre as features agrupadas.

## Decisão

**Decompomos em serviços separados, um por feature** (identity, achievements, player-data, cloud-save), com a **identity-service como fundação de auth** — os demais validam o JWT emitido por ela. Espelha o modelo per-feature do GameKit.

**Esta decisão é apenas sobre decomposição (quantos serviços e as fronteiras), NÃO sobre compute.** O compute de cada serviço é decidido no `plan.md`, regido pelo ADR-0005. A expectativa é que esses serviços (CRUD stateless por jogador) caiam naturalmente em **Lambda** — onde separar não custa idle (escala a zero), o que reforça a viabilidade da decomposição — mas a escolha vinculante é por serviço, não fixada aqui.

Fronteiras não são cravadas de uma vez: cada feature nova nasce como serviço próprio por default e só se junta a outra com motivo concreto de carga.

## Consequências

### Positivas
- Realismo de microsserviços (auth entre serviços, deploy/escala independentes); comparação fiel ao GameKit.

### Negativas / Trade-offs
- Mais scaffolds hexagonais; precisa resolver propagação/validação de auth entre serviços.

### Neutras / Implicações
- A identity-service é pré-requisito das demais; é o primeiro serviço da trilha.
- Compute permanece diferido (ADR-0005); este ADR não o fixa.

## Referências
- ADR-0005 (seleção de compute), ADR-0008 (tese)
- AWS GameKit — deploy por feature via CloudFormation
