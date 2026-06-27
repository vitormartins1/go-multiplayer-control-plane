# ADR-0007: Spec-Driven Development como metodologia

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** todos

---

## Contexto

O desenvolvimento é assistido por agente de IA, que começa cada sessão sem memória. Descrições vagas em chat geram drift entre intenção e código. É preciso uma fonte de verdade estável que alimente tanto o agente quanto os testes.

## Alternativas Consideradas

### Opção A: SDD (spec → plan → tasks → código)
- **Prós:** spec é fonte de verdade; artefatos versionados por feature; alinha agente e testes; permite parar/retomar sem perder estado.
- **Contras:** esforço inicial de escrever specs antes de codar.

### Opção B: TDD puro
- **Prós:** testes guiam o design.
- **Contras:** não captura comportamento/contrato de alto nível de forma legível para o agente; não substitui a spec.

### Opção C: Ad-hoc (direto ao código)
- **Prós:** rápido no curtíssimo prazo.
- **Contras:** drift, retrabalho, comportamento inventado pelo agente.

## Decisão

Adotamos **SDD**: cada feature tem `spec.md` (O QUE, em EARS) → `plan.md` (COMO) → `tasks.md` (trabalho atômico) → código → testes. A spec lidera; mudança de requisito edita a spec primeiro e regenera em cascata. TDD e SDD são complementares: os "Casos de Teste Obrigatórios" da spec viram os unit tests.

## Consequências

### Positivas
- Código consistente com a intenção; checklist vivo; ótimo para trabalho com agente de IA.

### Negativas / Trade-offs
- Disciplina de escrever spec antes de implementar.

### Neutras / Implicações
- Specs ficam em `services/<nome>/specs/NNN-slug/` (pasta por feature, numeração por serviço).

## Referências
- GitHub Spec Kit — github.com/github/spec-kit
- docs/sdd/SDD-GUIDE.md
