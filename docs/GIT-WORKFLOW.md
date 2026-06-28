# Git Workflow — game-backend

> Padrão de branches e commits do monorepo. Decisão registrada no **ADR-0013**.
> Objetivo: deixar claro **qual serviço** foi tocado, manter a `main` sempre integrável e ter unidades revisáveis.

## Modelo: trunk-based com feature branches curtas

- **`main`** é o trunk: sempre verde (build + testes passando) e integrável.
- Cada unidade de trabalho vive numa **branch curta**, mergeada de volta na `main` ao concluir.
- Nada quebrado entra na `main`.

## Nomenclatura de branches

```
<tipo>/<serviço-ou-área>/<slug>
```

| Exemplo | Quando |
|---|---|
| `feat/identity/001-auth-core` | feature SDD (slug = a pasta `NNN-slug` da spec) |
| `feat/leaderboard/001-ranking` | feature SDD de outro serviço |
| `fix/identity/jwks-kid` | correção |
| `refactor/session/registry` | refactor |
| `docs/feature-map` · `chore/ci` | docs/manutenção (sem serviço) |

Para features SDD, o slug **espelha a pasta da spec** — rastreabilidade direta entre branch, spec e código.

## Commits — Conventional Commits com escopo de serviço

```
<tipo>(<escopo>): <assunto no imperativo>
```

- **tipos:** `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `build`, `ci`, e os do SDD: `spec`, `plan`
- **escopo:** o serviço (`identity`, `leaderboard`, ...) ou área (`infra`, `adr`, `docs`)
- Exemplos: `feat(identity): implementa token issuer Ed25519` · `test(identity): cobre os casos da spec no service`
- Rodapé `Co-Authored-By:` quando aplicável.

**Granularidade:** ~1 commit por task/fase do `tasks.md`. Cada commit deixa o build verde.

## Fluxo de uma feature

```bash
# 1. branch a partir da main
git checkout main && git pull        # (pull só se houver remoto)
git checkout -b feat/identity/001-auth-core

# 2. implementar task por task, commitando por fase
git commit -m "feat(identity): ..."

# 3. garantir verde
go test ./... -race -cover && go build ./...

# 4. mergear na main preservando o histórico da feature
git checkout main
git merge --no-ff feat/identity/001-auth-core
git branch -d feat/identity/001-auth-core
```

- **`--no-ff`** mantém o agrupamento da feature na história da `main` (bom para auditar como cada feature evoluiu). Alternativa: *squash* para uma `main` mais terse — preferimos `--no-ff` pela rastreabilidade.
- **Com remoto GitHub (futuro):** em vez do merge local, abrir **PR**, CI verde, merge — ótimo para praticar review. O CI seletivo por path (`services/<svc>/**`) casa com o escopo da branch/commit.

## Releases

Tags/versionamento de release ficam para quando houver deploy (futuro).
