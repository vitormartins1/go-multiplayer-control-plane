# ADR-0013: Estratégia de branches e commits (trunk-based + feature branches)

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** todos (processo)

---

## Contexto

Ao iniciar a implementação de código (identity-service), o monorepo precisa de um modelo de versionamento padronizado: que torne claro **qual serviço** uma mudança afeta, mantenha a `main` sempre integrável e produza unidades revisáveis. Desenvolvimento solo, repositório local (sem remoto por ora, mas com GitHub previsto).

## Alternativas Consideradas

### Opção A: Trunk-based + feature branches curtas
- **Prós:** simples; `main` estável; branch/commit deixam claro o serviço afetado (casa com CI seletivo por path); migra naturalmente para PRs quando houver GitHub.
- **Contras:** exige disciplina de branch/merge.

### Opção B: Git Flow (develop/release/hotfix)
- **Prós:** estrutura para releases paralelos.
- **Contras:** cerimônia excessiva para solo/monorepo; branches de longa duração contra o espírito incremental do projeto.

### Opção C: Commit direto na `main` sempre
- **Prós:** zero overhead.
- **Contras:** sem isolamento; `main` pode quebrar; sem unidade revisável por feature.

## Decisão

Adotamos **trunk-based com feature branches curtas**. Uma branch por feature SDD, nomeada `feat/<serviço>/<NNN-slug>` (espelhando a pasta da spec). **Conventional Commits** com escopo de serviço (`feat(identity): ...`). Merge na `main` com `--no-ff` (preserva o histórico da feature); `main` sempre verde. Detalhes práticos em `docs/GIT-WORKFLOW.md`.

Quando houver remoto GitHub, o merge vira **PR** (com CI seletivo por path), sem mudar o modelo.

## Consequências

### Positivas
- `main` estável e integrável; rastreabilidade branch↔spec↔código; clareza de serviço afetado; pronto para PR-based.

### Negativas / Trade-offs
- Disciplina de branch/merge; `--no-ff` deixa merge commits na `main` (aceitável, e informativo).

### Neutras / Implicações
- Docs de processo afetam o agente: ver guardrail no `CLAUDE.md`.
- Releases/tags ficam para a fase de deploy.

## Referências
- Trunk-Based Development — trunkbaseddevelopment.com
- Conventional Commits — conventionalcommits.org
