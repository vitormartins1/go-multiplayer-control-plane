# CLAUDE.md — game-backend monorepo

Backend de jogos multiplayer em Go. Monorepo de **módulo único** (um `go.mod` na raiz — ver ADR-0001); cada serviço é um binário/imagem Docker independente.
Tese do projeto: reimplementar serviços de jogo da AWS (GameKit/GameLift) em Go sobre compute barato e comparar custo/performance (ADR-0008).
Metodologia: Spec-Driven Development (SDD). Leia `docs/sdd/SDD-GUIDE.md` antes de implementar qualquer serviço.

## Stack

- **Linguagem:** Go 1.22+ — **módulo único** na raiz (ainda não multi-módulo; ver ADR-0001)
- **Arquitetura interna:** Hexagonal (domain / service / repository / transport)
- **API:** REST com `net/http` (identity, achievements, player-data, cloud-save, leaderboard, analytics) | gRPC streaming (matchmaking, session)
- **Banco local:** PostgreSQL via `pgx/v5` (escolha de banco por serviço — ver ADR-0005)
- **Cloud:** AWS — compute por serviço (Lambda/Fargate/ECS-EC2/EC2/EKS — ver ADR-0005), DynamoDB, SQS, API Gateway, ECR
- **IaC:** Terraform (local → LocalStack, prod → AWS)
- **Observabilidade:** Prometheus + Grafana + OpenTelemetry
- **CI/CD:** GitHub Actions (deploy seletivo por serviço)

## Comandos essenciais

```bash
# Rodar todos os serviços localmente
docker compose -f infra/docker/docker-compose.yml up

# Rodar um serviço específico
cd services/identity-service && go run ./cmd/server/

# Testes com race detector
cd services/identity-service && go test ./... -v -race -cover

# Build
cd services/identity-service && go build ./cmd/server/...

# Lint
golangci-lint run ./...

# Terraform local (LocalStack)
cd infra/terraform/environments/local && terraform init && terraform apply

# Terraform prod (requer AWS credentials)
cd infra/terraform/environments/prod/identity-service && terraform init && terraform apply
```

## Estrutura dos serviços

Cada serviço em `services/<nome>-service/` segue a mesma estrutura (todos compartilham o `go.mod` único da raiz — não há `go.mod` por serviço; ver ADR-0001):

```
cmd/server/main.go       ← entrypoint, wire-up de dependências
internal/domain/         ← tipos puros + interfaces (Ports)
internal/service/        ← regras de negócio (só usa interfaces do domain)
internal/repository/     ← acesso a dados (implementa interfaces do domain)
internal/transport/      ← handlers HTTP ou gRPC (traduz para/de domínio)
migrations/              ← SQL migrations versionadas
specs/
  NNN-nome-feature/        ← uma pasta por feature, numerada por serviço
    spec.md                ← O QUE construir (EARS, comportamento)
    plan.md                ← COMO construir (decisões técnicas, arquivos)
    tasks.md               ← tarefas atômicas ([x] marcado conforme implementa)
```

## Regras arquiteturais

- `service/` NUNCA importa `transport/` ou `repository/` — só interfaces de `domain/`
- `domain/` tem zero dependências externas (sem frameworks, sem AWS SDK, sem `net/http`)
- Toda função que faz I/O recebe `ctx context.Context` como primeiro parâmetro
- Erros sempre wrappados: `fmt.Errorf("identity service: create: %w", err)`
- Goroutines SEMPRE com context para cancelamento e `sync.WaitGroup` para lifecycle
- Logging com `slog` em JSON: `slog.Info("event", slog.String("key", val))`

## Fluxo SDD (obrigatório)

Cada feature segue três fases com artefatos separados. Nunca pule fases sem confirmação.

1. **spec.md** — verifique se existe antes de qualquer implementação. Se não existir, pergunte ao usuário. Nunca invente comportamento.
2. **plan.md** — se spec existe mas plan não, ofereça gerar antes de implementar
3. **tasks.md** — se plan existe mas tasks não, ofereça gerar antes de implementar
4. **Implementação** — execute task por task, marcando `[x]` conforme conclui
5. Se encontrar ambiguidade não coberta pela spec → pare e pergunte, não chute

Guia completo: `docs/sdd/SDD-GUIDE.md`

## Padrões de teste

- Nomenclatura: `TestNomeDaFuncao_Cenario_ResultadoEsperado`
- Mocks: `testify/mock` (nunca struct real em unit test)
- Race detector: sempre rodar com `-race`
- Cobertura mínima: 80% no `service/` e `domain/`

## Convenções de documentação

- Diagramas (fluxos, sequências, relações, estados) em **Mermaid** — GitHub e o preview local renderizam nativo. Não use ASCII art para diagramas.
- Listagens de estrutura de pastas/arquivos podem permanecer em bloco de código texto (Mermaid não representa bem árvore de arquivos).
- Aprendizados de Go relevantes vão em `docs/GO-LEARNINGS.md` (exemplos comentados), atualizados conforme surgem no desenvolvimento.

## Git e versionamento

Trabalhe em **feature branches** `feat/<serviço>/<NNN-slug>` (uma por feature SDD), nunca direto na `main`. **Conventional Commits** com escopo de serviço (`feat(identity): ...`). `main` sempre verde. Guia completo: `docs/GIT-WORKFLOW.md` (ADR-0013).

## ADRs — Architecture Decision Records

Leia `docs/adr/ADR-GUIDE.md` para o guia completo. Resumo do comportamento esperado:

**Detecção automática:** Monitore sinais de decisão arquitetural durante a sessão — escolhas entre tecnologias, padrões, protocolos, infra. Quando detectar, proponha criar um ADR de forma concisa, sem bloquear o fluxo.

**Antes de implementar algo arquitetural:** Verifique `docs/adr/README.md`. Se a implementação conflitar com um ADR aceito, avise o usuário antes de prosseguir.

**Fluxo de criação:**
1. Detectou decisão → sinalize brevemente e pergunte se quer registrar
2. Rascunhe o ADR com contexto, alternativas e trade-offs reais
3. Apresente o rascunho para revisão
4. Só grave em `docs/adr/NNNN-slug.md` após confirmação explícita
5. Atualize `docs/adr/README.md`

**Não crie ADR sem aprovação do usuário. Não insista se ele disser "depois".**

## O que NÃO fazer

- Não modifique `shared/` sem consultar o usuário — afeta todos os serviços
- Não misture lógica de negócio em `transport/` (handler deve só traduzir)
- Não ignore erros com `_`
- Não crie goroutines sem controle de lifecycle
- Não adicione dependências externas em `domain/`
- Não faça deploy direto — use o pipeline CI/CD
- Não grave ADRs sem confirmação do usuário
- Não crie `go.mod` por serviço — o repo é módulo único (ADR-0001)
- Não implemente gameplay realtime em Go — o data plane é o Unity Dedicated Server, em repo separado (ADR-0010)

## Referências detalhadas

- Arquitetura completa: `docs/ROADMAP.md`
- Guia SDD: `docs/sdd/SDD-GUIDE.md`
- Guia ADR: `docs/adr/ADR-GUIDE.md`
- Índice de ADRs: `docs/adr/README.md`
- Spec de cada serviço: `services/<nome>/specs/`
