# Game Backend em Go — Roadmap & Arquitetura
> Monorepo de microsserviços para backend de jogos multiplayer
> Autor: Vitor (C130N) | Stack: Go · gRPC · Terraform · AWS · LocalStack · Grafana

---

## Visão Geral

Este projeto serve dois propósitos simultâneos:

1. **Aprender Go** de forma estruturada e progressiva, contextualizado com C#/.NET
2. **Construir infraestrutura reutilizável** para jogos próprios futuros

O backend é um **monorepo com microsserviços independentes**: cada serviço tem seu próprio binário, módulo Go, Dockerfile, infra Terraform e ciclo de deploy separado.

A metodologia de desenvolvimento adotada é **Spec-Driven Development (SDD)**: specs escritas antes do código, usadas como fonte de verdade para o agente de IA e para os testes.

---

## Estrutura do Monorepo

```
game-backend/                              ← raiz do monorepo
│
├── services/
│   ├── player-service/                    ← microsserviço: CRUD de jogadores
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── domain/                    ← tipos de negócio puros, sem dependências externas
│   │   │   │   ├── player.go
│   │   │   │   └── errors.go
│   │   │   ├── service/                   ← regras de negócio (usa interfaces do domain)
│   │   │   │   ├── player_service.go
│   │   │   │   └── player_service_test.go
│   │   │   ├── repository/                ← acesso a dados (implementa interfaces do domain)
│   │   │   │   ├── postgres_repository.go
│   │   │   │   └── postgres_repository_test.go
│   │   │   └── transport/                 ← handlers HTTP (entram, saem, não têm lógica)
│   │   │       ├── http_handler.go
│   │   │       └── http_handler_test.go
│   │   ├── migrations/                    ← SQL migrations versionadas
│   │   ├── specs/                         ← SDD: specs antes do código
│   │   │   ├── player-service.spec.md
│   │   │   └── api-contract.md
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   ├── achievements-service/
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── domain/
│   │   │   ├── service/
│   │   │   ├── repository/
│   │   │   └── transport/
│   │   ├── migrations/
│   │   ├── specs/
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   ├── analytics-service/
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── domain/
│   │   │   ├── service/
│   │   │   ├── repository/
│   │   │   ├── transport/
│   │   │   └── worker/                    ← goroutine workers assíncronos (exclusivo deste serviço)
│   │   ├── specs/
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   └── matchmaking-service/
│       ├── cmd/server/main.go
│       ├── internal/
│       │   ├── domain/
│       │   ├── service/
│       │   ├── queue/                     ← fila com goroutines + channels
│       │   └── transport/                 ← gRPC server (não HTTP)
│       ├── proto/                         ← .proto definitions (local ao serviço)
│       ├── specs/
│       ├── Dockerfile
│       └── go.mod
│
├── shared/                                ← código compartilhado (apenas utilitários)
│   ├── pkg/
│   │   ├── logger/                        ← slog wrapper padronizado
│   │   ├── middleware/                    ← auth, metrics, tracing
│   │   ├── config/                        ← leitura de env vars
│   │   └── telemetry/                     ← OpenTelemetry setup
│   └── go.mod                             ← módulo separado: github.com/c130n/game-backend/shared
│
├── infra/
│   ├── terraform/
│   │   ├── modules/                       ← módulos reutilizáveis
│   │   │   ├── ecs-service/               ← ECS Fargate genérico, parametrizável por serviço
│   │   │   ├── dynamodb/
│   │   │   ├── sqs/
│   │   │   ├── api-gateway/
│   │   │   └── monitoring/                ← CloudWatch + Budgets
│   │   ├── environments/
│   │   │   ├── local/                     ← aponta para LocalStack
│   │   │   │   └── main.tf
│   │   │   └── prod/                      ← AWS real, state no S3
│   │   │       ├── shared/                ← VPC, ECR, IAM (provisionado uma vez)
│   │   │       ├── player-service/        ← infra isolada: ECS + DynamoDB
│   │   │       ├── achievements-service/
│   │   │       ├── analytics-service/     ← infra isolada: ECS + SQS (mais tasks, escala diferente)
│   │   │       └── matchmaking-service/   ← infra isolada: ECS (conexões longas, gRPC)
│   │   └── shared-vars.tf
│   └── docker/
│       ├── docker-compose.yml             ← ambiente local completo
│       └── prometheus.yml
│
├── docs/                                  ← documentação geral do projeto
│   ├── ROADMAP.md                         ← este arquivo
│   ├── ARCHITECTURE.md                    ← decisões arquiteturais (ADRs)
│   ├── adr/                               ← Architecture Decision Records
│   │   ├── 001-monorepo.md
│   │   ├── 002-hexagonal-arch.md
│   │   └── 003-grpc-matchmaking.md
│   └── sdd/                              ← guia SDD do projeto
│       └── SDD-GUIDE.md
│
├── .github/
│   └── workflows/
│       ├── ci-player.yml
│       ├── ci-achievements.yml
│       ├── ci-analytics.yml
│       ├── ci-matchmaking.yml
│       └── cd.yml                         ← deploy seletivo por serviço modificado
│
├── CLAUDE.md                              ← contexto do agente (raiz do monorepo)
├── AGENTS.md                              ← cross-tool (Cursor, Codex, etc.)
└── README.md
```

---

## Arquitetura Interna de Cada Serviço — Hexagonal

Todos os serviços seguem **Hexagonal Architecture** (Ports & Adapters), que é o padrão de mercado para microsserviços Go:

```
┌─────────────────────────────────────────────┐
│                   transport/                │  ← Adapter de entrada (HTTP/gRPC)
│         handler recebe request, chama       │
│         service via interface (Port)        │
└─────────────────┬───────────────────────────┘
                  │ interface (Port)
┌─────────────────▼───────────────────────────┐
│                   service/                  │  ← Core: regras de negócio puras
│         não conhece HTTP nem banco          │
│         depende apenas de interfaces        │
└─────────────────┬───────────────────────────┘
                  │ interface (Port)
┌─────────────────▼───────────────────────────┐
│                 repository/                 │  ← Adapter de saída (banco, cache, SQS)
│         implementa a interface do domain    │
│         conhece SQL/DynamoDB/AWS SDK        │
└─────────────────────────────────────────────┘
```

**Regra principal:** o `service/` nunca importa `transport/` nem `repository/` diretamente. Só trabalha com interfaces declaradas em `domain/`. Isso torna unit testing trivial — basta mockar a interface.

### Exemplo em código

```go
// domain/player.go — tipos puros, zero dependências externas
package domain

type Player struct {
    ID       string
    Name     string
    Rating   int
    CreatedAt time.Time
}

// Port de saída — interface que o service usa
type PlayerRepository interface {
    FindByID(ctx context.Context, id string) (*Player, error)
    Save(ctx context.Context, p *Player) error
}

// Port de entrada — interface que o transport usa
type PlayerService interface {
    GetPlayer(ctx context.Context, id string) (*Player, error)
    CreatePlayer(ctx context.Context, name string) (*Player, error)
}
```

```go
// service/player_service.go — regras de negócio
package service

type playerService struct {
    repo domain.PlayerRepository  // injeta a interface, não a implementação
}

func NewPlayerService(repo domain.PlayerRepository) domain.PlayerService {
    return &playerService{repo: repo}
}

func (s *playerService) CreatePlayer(ctx context.Context, name string) (*domain.Player, error) {
    if name == "" {
        return nil, domain.ErrInvalidName  // erro de domínio, não HTTP
    }
    p := &domain.Player{ID: uuid.New().String(), Name: name, Rating: 1000}
    return p, s.repo.Save(ctx, p)
}
```

```go
// transport/http_handler.go — só traduz HTTP ↔ domínio
package transport

func (h *Handler) CreatePlayer(w http.ResponseWriter, r *http.Request) {
    var req CreatePlayerRequest
    json.NewDecoder(r.Body).Decode(&req)

    player, err := h.service.CreatePlayer(r.Context(), req.Name)
    if err != nil {
        // mapeia erros de domínio → HTTP status codes
        writeError(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, player)
}
```

---

## Boas Práticas Go Adotadas

### Tratamento de erros
```go
// NUNCA: ignorar erro
result, _ := service.DoSomething()

// SEMPRE: tratar explicitamente
result, err := service.DoSomething()
if err != nil {
    return fmt.Errorf("player service: do something: %w", err)  // wrapping com contexto
}
```

### Interfaces pequenas (Go idiomático)
```go
// RUIM: interface gigante difícil de mockar
type PlayerRepository interface {
    FindByID(...) (*Player, error)
    FindByName(...) (*Player, error)
    Save(...) error
    Delete(...) error
    List(...) ([]*Player, error)
    // ...10 métodos mais
}

// BOM: interfaces pequenas, compostas quando necessário
type PlayerFinder interface {
    FindByID(ctx context.Context, id string) (*Player, error)
}
type PlayerWriter interface {
    Save(ctx context.Context, p *Player) error
}
```

### Context em toda chamada I/O
```go
// Toda função que faz I/O (banco, HTTP, gRPC) recebe ctx como primeiro parâmetro
func (r *repository) FindByID(ctx context.Context, id string) (*domain.Player, error) {
    // ctx carrega: deadline, cancelamento, tracing
}
```

### Logging estruturado (slog — padrão desde Go 1.21)
```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
logger.Info("player created",
    slog.String("player_id", player.ID),
    slog.String("name", player.Name),
    slog.Duration("latency", time.Since(start)),
)
```

### Goroutines — sempre com context e WaitGroup
```go
// NUNCA: goroutine solta sem controle de lifecycle
go processEvent(event)

// SEMPRE: com context + WaitGroup para graceful shutdown
func (w *Worker) Start(ctx context.Context) {
    var wg sync.WaitGroup
    for i := 0; i < w.concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            w.loop(ctx)  // para quando ctx for cancelado
        }()
    }
    wg.Wait()
}
```

### Testes
- Unit tests com `testing` nativo + `testify/assert`
- Mocks com `testify/mock` ou `gomock`
- Testes de integração com `testcontainers-go` (sobe banco real em Docker)
- Nomenclatura: `TestNomeDaFuncao_Cenario_ResultadoEsperado`

```go
func TestCreatePlayer_EmptyName_ReturnsError(t *testing.T) {
    mockRepo := new(MockPlayerRepository)
    svc := service.NewPlayerService(mockRepo)

    _, err := svc.CreatePlayer(context.Background(), "")

    assert.ErrorIs(t, err, domain.ErrInvalidName)
    mockRepo.AssertNotCalled(t, "Save")
}
```

---

## Spec-Driven Development (SDD)

### O que é

SDD é a metodologia onde a **spec é a fonte de verdade**, não o código. O fluxo é:

```
1. SPEC    → descreve O QUE o sistema faz (comportamento, contratos, regras)
2. PLAN    → descreve COMO implementar (tarefas atômicas derivadas da spec)
3. CODE    → gerado/escrito a partir do plan
4. TEST    → verifica que o código satisfaz a spec
```

O agente de IA (Claude Code) trabalha a partir das specs, não de descrições vagas. Isso produz código consistente com a intenção original e evita drift entre requisito e implementação.

### Estrutura de uma Spec

Cada microsserviço tem sua pasta `specs/` com:

```
services/player-service/specs/
├── player-service.spec.md     ← spec de comportamento (EARS format)
├── api-contract.md            ← contrato da API (OpenAPI-like em markdown)
└── data-model.md              ← modelo de dados e invariantes
```

### Formato EARS para regras (usado nas specs)

EARS (Easy Approach to Requirements Syntax) elimina ambiguidade:

```markdown
## Regras de Negócio — Player

- WHEN um novo jogador é criado, the system SHALL atribuir rating inicial de 1000
- IF o nome estiver vazio, the system SHALL retornar ErrInvalidName
- WHEN um jogador é buscado por ID inexistente, the system SHALL retornar ErrNotFound
- THE SYSTEM SHALL rejeitar nomes com menos de 2 ou mais de 50 caracteres
```

### Exemplo de spec completa

```markdown
# Spec: Player Service

## Contexto
Microsserviço responsável por criação, leitura e atualização de perfis de jogadores.

## Comportamento

### Criar jogador
- WHEN POST /players com nome válido → retorna 201 + player criado
- IF nome vazio ou < 2 chars → retorna 400 com ErrInvalidName
- IF nome > 50 chars → retorna 400 com ErrNameTooLong
- THE SYSTEM SHALL garantir unicidade de nome (case-insensitive)

### Buscar jogador
- WHEN GET /players/:id com ID existente → retorna 200 + player
- IF ID não existir → retorna 404 com ErrNotFound
- IF ID mal formatado → retorna 400 com ErrInvalidID

## Contratos de Interface

### PlayerRepository (Port de saída)
- FindByID(ctx, id) → (*Player, error)
- FindByName(ctx, name) → (*Player, error)
- Save(ctx, player) → error

## Casos de Teste Obrigatórios
- [ ] CreatePlayer com nome válido → sucesso
- [ ] CreatePlayer com nome vazio → ErrInvalidName
- [ ] CreatePlayer com nome duplicado → ErrDuplicateName
- [ ] GetPlayer com ID existente → sucesso
- [ ] GetPlayer com ID inexistente → ErrNotFound
```

### Fluxo de trabalho com Claude Code

```
1. Você escreve ou refina a spec em specs/*.spec.md
2. Você abre Claude Code e diz: "Implemente o player-service seguindo a spec em specs/player-service.spec.md"
3. Claude lê a spec + CLAUDE.md e gera o código alinhado com a arquitetura
4. Claude gera os testes a partir dos "Casos de Teste Obrigatórios" da spec
5. Se comportamento mudar: edite a spec → peça ao Claude para atualizar o código
```

---

## Infraestrutura com Terraform

### Princípio: cada serviço tem estado Terraform independente

```
infra/terraform/environments/prod/
├── shared/          → terraform init + apply (uma vez, VPC + ECR + IAM)
├── player-service/  → terraform init + apply (independente)
├── analytics-service/ → terraform init + apply (independente, mais tasks)
└── matchmaking-service/ → terraform init + apply (independente, gRPC)
```

### Local com LocalStack

```hcl
# environments/local/main.tf
provider "aws" {
  access_key                  = "test"
  secret_key                  = "test"
  region                      = "us-east-1"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  endpoints {
    dynamodb = "http://localhost:4566"
    sqs      = "http://localhost:4566"
    lambda   = "http://localhost:4566"
  }
}
```

### Escalabilidade independente por serviço

| Serviço | CPU/Mem | Min tasks | Max tasks | Métrica de escala |
|---|---|---|---|---|
| player-service | 256/512 | 1 | 3 | CPU % |
| achievements-service | 256/512 | 1 | 5 | Request count |
| analytics-service | 512/1024 | 1 | 10 | SQS queue depth |
| matchmaking-service | 512/1024 | 1 | 8 | Conexões ativas |

---

## Ambiente Local com Docker Compose

```yaml
services:
  player-service:
    build: ../../services/player-service
    ports: ["8081:8080"]
    environment:
      DB_HOST: postgres
      AWS_ENDPOINT: http://localstack:4566

  achievements-service:
    build: ../../services/achievements-service
    ports: ["8082:8080"]

  analytics-service:
    build: ../../services/analytics-service
    ports: ["8083:8080"]

  matchmaking-service:
    build: ../../services/matchmaking-service
    ports: ["50051:50051"]

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: gamebackend
      POSTGRES_USER: dev
      POSTGRES_PASSWORD: dev

  localstack:
    image: localstack/localstack
    ports: ["4566:4566"]
    environment:
      SERVICES: sqs,dynamodb,lambda,ecr

  prometheus:
    image: prom/prometheus
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    ports: ["9090:9090"]

  grafana:
    image: grafana/grafana
    ports: ["3000:3000"]
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    volumes:
      - ./grafana/dashboards:/var/lib/grafana/dashboards
```

---

## Observabilidade com Grafana

### Três pilares obrigatórios

1. **Logs** — `slog` com JSON handler → CloudWatch / Grafana Loki
2. **Métricas** — Prometheus client Go → Grafana dashboards
3. **Traces** — OpenTelemetry → Grafana Tempo (ou AWS X-Ray)

### Métricas por serviço no Grafana

| Dashboard | Métricas |
|---|---|
| API Overview | RPS, latência p50/p95/p99, error rate por serviço |
| Matchmaking | Tamanho da fila, tempo médio de espera, partidas formadas/min |
| Analytics | Eventos recebidos/s, eventos processados/s, lag da fila SQS |
| Go Runtime | Goroutines ativas, heap usage, GC pause duration |
| AWS FinOps | Custo por serviço (via CloudWatch Billing) |

---

## CI/CD com GitHub Actions

### CI por serviço (roda em paralelo)

```yaml
# .github/workflows/ci-player.yml
on:
  push:
    paths: ["services/player-service/**"]

jobs:
  test:
    steps:
      - uses: actions/setup-go@v5
        with: {go-version: '1.22'}
      - run: cd services/player-service && go test ./... -v -race -cover
      - run: cd services/player-service && go build ./cmd/server/...
```

### CD seletivo — só deploya o que mudou

```yaml
# .github/workflows/cd.yml
jobs:
  detect-changes:
    outputs:
      player:       ${{ steps.changes.outputs.player }}
      analytics:    ${{ steps.changes.outputs.analytics }}
      matchmaking:  ${{ steps.changes.outputs.matchmaking }}
    steps:
      - uses: dorny/paths-filter@v3
        id: changes
        with:
          filters: |
            player:       ["services/player-service/**"]
            analytics:    ["services/analytics-service/**"]
            matchmaking:  ["services/matchmaking-service/**"]

  deploy-analytics:
    needs: detect-changes
    if: needs.detect-changes.outputs.analytics == 'true'
    # build → ECR → ECS update apenas do analytics-service
```

---

## FinOps — Controle de Custo

| Serviço AWS | Uso | Estimativa/mês |
|---|---|---|
| ECS Fargate | 4 serviços × tarefa mínima | ~$15–25 |
| DynamoDB on-demand | Baixo volume inicial | ~$1–3 |
| SQS | analytics events | Gratuito (< 1M/mês) |
| API Gateway | REST endpoints | ~$3–5 |
| ECR | imagens Docker | ~$1 |
| CloudWatch | logs + alarms | ~$2–3 |
| **Total estimado** | **fase de estudo** | **~$22–37/mês** |

**AWS Budget configurado via Terraform com alerta em 80% de $40/mês.**

---

## Roadmap de Estudo — Fases

### Fase 1 — Fundamentos Go (2–3 semanas)
- [ ] Tour of Go + Go by Example
- [ ] Structs, interfaces, ponteiros, error handling
- [ ] Unit tests com `testing` + `testify`
- [ ] Primeira spec: `player-service.spec.md`

### Fase 2 — Player Service + Hexagonal Arch (3–4 semanas)
- [ ] Estrutura hexagonal: domain / service / repository / transport
- [ ] API REST com `net/http` puro
- [ ] PostgreSQL com `pgx`
- [ ] Docker Compose local
- [ ] CI no GitHub Actions

### Fase 3 — Terraform + LocalStack (2 semanas)
- [ ] Terraform básico: providers, resources, modules, state
- [ ] LocalStack: DynamoDB + SQS
- [ ] Módulo reutilizável `ecs-service`
- [ ] Ambiente prod na AWS

### Fase 4 — Analytics + Goroutines (2–3 semanas)
- [ ] Worker pool com goroutines + channels
- [ ] `context` com cancelamento e deadline
- [ ] Integração SQS via AWS SDK Go v2
- [ ] Métricas Prometheus + dashboard Grafana

### Fase 5 — CI/CD completo (1–2 semanas)
- [ ] CD seletivo por serviço
- [ ] Build Docker + push ECR
- [ ] Deploy ECS Fargate
- [ ] AWS Budgets + CloudWatch Alarms

### Fase 6 — Matchmaking gRPC (3–4 semanas)
- [ ] Protobuf: definir `.proto`, gerar código Go
- [ ] gRPC server com streaming bidirecional
- [ ] Fila de matchmaking com goroutines + channels
- [ ] Testes de integração gRPC

---

## Referências

| Recurso | URL |
|---|---|
| Tour of Go | tour.golang.org |
| Go by Example | gobyexample.com |
| Effective Go | go.dev/doc/effective_go |
| golang-standards/project-layout | github.com/golang-standards/project-layout |
| AWS SDK Go v2 | pkg.go.dev/github.com/aws/aws-sdk-go-v2 |
| gRPC Go | grpc.io/docs/languages/go |
| LocalStack | docs.localstack.cloud |
| Terraform AWS Provider | registry.terraform.io/providers/hashicorp/aws |
| Prometheus Go Client | prometheus.io/docs/guides/go-application |
| OpenTelemetry Go | opentelemetry.io/docs/languages/go |
| SDD Guide (Thoughtworks) | thoughtworks.com/insights/blog/spec-driven-development |
