# AGENTS.md — game-backend monorepo

> Cross-tool agent instructions (Claude Code, Cursor, Codex CLI, Gemini CLI, Zed, opencode)
> Conteúdo alinhado com CLAUDE.md. Em caso de conflito, CLAUDE.md tem precedência no Claude Code.

## O que é este projeto

Monorepo de microsserviços em Go para backend de jogos multiplayer.
Cada serviço em `services/` é independente: módulo Go próprio, Dockerfile próprio, infra Terraform própria.

## Metodologia: Spec-Driven Development

**Toda implementação começa pela spec.** Não escreva código antes de ler `services/<nome>/specs/*.spec.md`.
Se a spec não existir, crie-a junto com o usuário antes de qualquer código.

## Arquitetura interna (todos os serviços)

Hexagonal Architecture — camadas com dependências unidirecionais:

```
transport/ → service/ → repository/
               ↑               ↑
           (via interfaces de domain/)
```

- `domain/` — tipos e interfaces puras. Zero imports externos.
- `service/` — lógica de negócio. Só usa interfaces de `domain/`.
- `repository/` — acesso a dados. Implementa interfaces de `domain/`.
- `transport/` — HTTP handlers ou gRPC server. Só traduz, sem lógica.

## Regras invioláveis

1. `service/` não importa `transport/` nem `repository/` diretamente
2. Todo I/O recebe `ctx context.Context` como primeiro parâmetro
3. Erros sempre com contexto: `fmt.Errorf("camada: operação: %w", err)`
4. Goroutines sempre com context para cancelamento
5. Logging com `slog` em JSON
6. Testes: nomenclatura `TestFunc_Cenario_Resultado`, mocks com `testify/mock`

## Estrutura de arquivos

```
services/<nome>-service/
  cmd/server/main.go     ← entrypoint e wire-up
  internal/domain/       ← tipos + interfaces
  internal/service/      ← lógica de negócio
  internal/repository/   ← acesso a dados
  internal/transport/    ← HTTP/gRPC handlers
  migrations/            ← SQL migrations
  specs/                 ← specs SDD (leia primeiro)
  go.mod                 ← módulo independente
```

## Comandos de build e teste

```bash
# Todos os serviços locais
docker compose -f infra/docker/docker-compose.yml up

# Serviço específico
cd services/<nome>-service && go run ./cmd/server/

# Testes (sempre com -race)
cd services/<nome>-service && go test ./... -v -race -cover

# Lint
golangci-lint run ./...
```

## ADRs

Leia `docs/adr/ADR-GUIDE.md`. Durante a sessão, monitore decisões arquiteturais e proponha criar ADRs quando detectar escolhas entre tecnologias, padrões ou infra. Só grave o arquivo após confirmação explícita do usuário. Antes de implementar algo arquitetural, verifique `docs/adr/README.md` para consistência com decisões existentes.

## O que não fazer

- Não modifique `shared/` sem aprovação — impacta todos os serviços
- Não coloque lógica de negócio em `transport/`
- Não ignore erros (`_`)
- Não escreva código sem ler a spec primeiro
- Não faça deploy manual — use os pipelines em `.github/workflows/`

## Documentação

- Roadmap completo: `docs/ROADMAP.md`
- Guia SDD: `docs/sdd/SDD-GUIDE.md`
- Guia ADR: `docs/adr/ADR-GUIDE.md`
- Índice de ADRs: `docs/adr/README.md`
