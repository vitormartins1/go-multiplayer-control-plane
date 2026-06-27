# CLAUDE.md — game-backend monorepo

Backend de jogos multiplayer em Go. Monorepo com microsserviços independentes.
Metodologia: Spec-Driven Development (SDD). Leia `docs/sdd/SDD-GUIDE.md` antes de implementar qualquer serviço.

## Stack

- **Linguagem:** Go 1.22+
- **Arquitetura interna:** Hexagonal (domain / service / repository / transport)
- **API:** REST com `net/http` (player, achievements, analytics) | gRPC (matchmaking)
- **Banco local:** PostgreSQL via `pgx/v5`
- **Cloud:** AWS — ECS Fargate, DynamoDB, SQS, API Gateway, ECR
- **IaC:** Terraform (local → LocalStack, prod → AWS)
- **Observabilidade:** Prometheus + Grafana + OpenTelemetry
- **CI/CD:** GitHub Actions (deploy seletivo por serviço)

## Comandos essenciais

```bash
# Rodar todos os serviços localmente
docker compose -f infra/docker/docker-compose.yml up

# Rodar um serviço específico
cd services/player-service && go run ./cmd/server/

# Testes com race detector
cd services/player-service && go test ./... -v -race -cover

# Build
cd services/player-service && go build ./cmd/server/...

# Lint
golangci-lint run ./...

# Terraform local (LocalStack)
cd infra/terraform/environments/local && terraform init && terraform apply

# Terraform prod (requer AWS credentials)
cd infra/terraform/environments/prod/player-service && terraform init && terraform apply
```

## Estrutura dos serviços

Cada serviço em `services/<nome>-service/` segue a mesma estrutura:

```
cmd/server/main.go       ← entrypoint, wire-up de dependências
internal/domain/         ← tipos puros + interfaces (Ports)
internal/service/        ← regras de negócio (só usa interfaces do domain)
internal/repository/     ← acesso a dados (implementa interfaces do domain)
internal/transport/      ← handlers HTTP ou gRPC (traduz para/de domínio)
migrations/              ← SQL migrations versionadas
specs/                   ← specs SDD (leia antes de qualquer implementação)
```

## Regras arquiteturais

- `service/` NUNCA importa `transport/` ou `repository/` — só interfaces de `domain/`
- `domain/` tem zero dependências externas (sem frameworks, sem AWS SDK, sem `net/http`)
- Toda função que faz I/O recebe `ctx context.Context` como primeiro parâmetro
- Erros sempre wrappados: `fmt.Errorf("player service: create: %w", err)`
- Goroutines SEMPRE com context para cancelamento e `sync.WaitGroup` para lifecycle
- Logging com `slog` em JSON: `slog.Info("event", slog.String("key", val))`

## Fluxo SDD (obrigatório)

1. Leia a spec em `specs/*.spec.md` antes de implementar
2. Se a spec não existir ou estiver incompleta, pergunte ao usuário antes de prosseguir
3. Implemente seguindo exatamente os contratos e regras da spec
4. Gere os casos de teste listados na seção "Casos de Teste Obrigatórios" da spec
5. Se precisar desviar da spec, documente o motivo em comentário e avise o usuário

## Padrões de teste

- Nomenclatura: `TestNomeDaFuncao_Cenario_ResultadoEsperado`
- Mocks: `testify/mock` (nunca struct real em unit test)
- Race detector: sempre rodar com `-race`
- Cobertura mínima: 80% no `service/` e `domain/`

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

## Referências detalhadas

- Arquitetura completa: `docs/ROADMAP.md`
- Guia SDD: `docs/sdd/SDD-GUIDE.md`
- Guia ADR: `docs/adr/ADR-GUIDE.md`
- Índice de ADRs: `docs/adr/README.md`
- Spec de cada serviço: `services/<nome>/specs/`
