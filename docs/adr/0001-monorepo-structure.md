# ADR-0001: Monorepo Go iniciando como módulo único

**Status:** Accepted
**Data:** 2026-06-27
**Serviço(s) afetado(s):** todos

---

## Contexto

O projeto é um monorepo de microsserviços Go para backend de jogos. Cada serviço deve ser deployável de forma independente (binário e imagem Docker próprios). Há código compartilhado fino (logger, config, middleware, telemetry).

Em Go, a unidade de dependências é o **módulo** (`go.mod`). A pergunta estrutural é: um módulo único na raiz, ou um módulo por serviço (multi-módulo)? O autor é novo em Go e quer seguir boas práticas sem over-engineering, e confirmou **não** ter necessidade de versões de dependência divergentes entre serviços.

## Alternativas Consideradas

### Opção A: Módulo único (um `go.mod` na raiz)
- **Prós:** simples e idiomático; `go test ./...` da raiz roda tudo; refatorar entre serviços é trivial; zero plumbing (sem `go.work`/`replace`); builds Docker simples. Cada serviço continua sendo binário/imagem independentes.
- **Contras:** um único conjunto de dependências para todos os serviços (um `go.sum`). Não permite versionar serviços como módulos publicáveis — irrelevante, pois deployamos binários.

### Opção B: Multi-módulo + `go.work`
- **Prós:** independência real de dependências por serviço; mecânica de módulos "enterprise". `go.work` (Go 1.18+) é a forma oficial de costurar módulos no dev local.
- **Contras:** atrito real no build Docker e CI (resolver `shared` via `go.work`/`replace`); plumbing que distrai do aprendizado da linguagem. Benefício principal (versões divergentes) não é necessário agora.

### Opção C: Polyrepo (um repo por serviço)
- **Prós:** isolamento total.
- **Contras:** perde a vantagem de evolução conjunta; overhead de coordenação; contra a premissa de monorepo.

## Decisão

Começamos com **módulo único** (um `go.mod` na raiz, `github.com/c130n/game-backend`). Cada serviço é um binário independente sob `services/<nome>/cmd/` e uma imagem Docker própria; `shared/` é um pacote interno importado diretamente.

A migração para **multi-módulo + `go.work`** fica reservada para quando surgir uma necessidade concreta (ex: versões de dependência divergentes, times separados, ou o repo crescer a ponto de o build único pesar). A direção módulo-único → multi-módulo é a fácil; o caminho inverso é mais custoso. `go.work` é a abordagem oficial e será adotada nesse momento, não antes.

## Consequências

### Positivas
- Curva de entrada baixa; foco na linguagem, não em encanamento de módulos.
- Builds e CI simples; refactor cross-serviço trivial.

### Negativas / Trade-offs
- Todos os serviços compartilham um conjunto de versões de dependência.
- Quando a migração para multi-módulo acontecer, será um trabalho deliberado (porém mecânico).

### Neutras / Implicações
- `CLAUDE.md` e `AGENTS.md` (que mencionavam `go.mod` por serviço) devem ser atualizados para refletir módulo único.
- O `internal/` de cada serviço dá encapsulamento entre serviços naturalmente (regra `internal` do Go).

## Referências
- Go Modules Reference — go.dev/ref/mod
- Go Workspaces (go.work) — go.dev/doc/tutorial/workspaces
