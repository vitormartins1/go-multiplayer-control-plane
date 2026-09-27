# Contribuindo

Obrigado pelo interesse. Este guia resume como o projeto trabalha; os detalhes estão nos documentos linkados.

## Antes de escrever código

1. **Existe spec?** Toda feature nasce de uma spec em `services/<serviço>/specs/NNN-slug/spec.md`, com
   comportamento e critérios de aceite em EARS. Sem spec, escreva uma e discuta antes de implementar. Fatia
   pequena (campo novo, passthrough) pede só uma spec curta; fatia substancial pede também `plan.md` e
   `tasks.md`. [Guia SDD](docs/sdd/SDD-GUIDE.md).
2. **Toca arquitetura?** Confira os [ADRs](docs/adr/README.md). Se a mudança contradiz um ADR aceito, o caminho
   é propor um ADR novo, não contornar o antigo. [Como escrever um ADR](docs/adr/ADR-GUIDE.md).
3. **Muda o contrato com o jogo?** Os `.proto` só aceitam mudanças **aditivas** (o CI roda `buf breaking`).
   Registre a mudança em [`docs/contracts/CHANGELOG.md`](docs/contracts/CHANGELOG.md).

## Regras do código

- **Hexagonal**: `domain/` sem dependências externas; `service/` só enxerga interfaces do `domain/`;
  `transport/` só traduz (nenhuma regra de negócio em handler).
- Toda função com I/O recebe `ctx context.Context` primeiro. Erros sempre embrulhados com contexto
  (`fmt.Errorf("session service: open world: %w", err)`); nenhum erro ignorado com `_` sem justificativa.
- Goroutine sempre com cancelamento por `context` e ciclo de vida controlado (`sync.WaitGroup`).
- Logs com `slog` em JSON. Nada de segredo (chave Steam, `world_token`, senha) em log.
- `shared/` afeta todos os serviços: mudanças ali pedem discussão antes.
- Serviço novo entra na observabilidade antes de fechar a feature: métricas RED, scrape, tracing e exporter do
  banco ([checklist](docs/OBSERVABILITY.md)).

## Testes

- Nome: `TestNomeDaFuncao_Cenario_ResultadoEsperado`.
- Unitários com `testify/mock`; integração com testcontainers sob a tag `integration`.
- Sempre com `-race`. Cobertura mínima de 80% em `service/` e `domain/`.

```bash
go build ./... && go vet ./...
go test ./... -race -cover
go test -tags=integration ./...     # precisa de Docker
golangci-lint run ./...             # v2.12.2
cd services/admin-ui && npm run lint && npm test && npm run build
```

## Branches, commits e PR

- Branch curta a partir da `main`: `feat/<serviço>/<NNN-slug>`, `fix/<serviço>/<slug>`, `docs/<slug>`.
- [Conventional Commits](https://www.conventionalcommits.org/pt-br/) com o serviço no escopo:
  `feat(session): heartbeat do mundo (spec 032)`. Cada commit deixa o build verde.
- O PR diz qual spec implementa, como foi validado e o que ficou de fora. A `main` fica sempre verde.

Guia completo: [`docs/GIT-WORKFLOW.md`](docs/GIT-WORKFLOW.md) (ADR-0013).

## Documentação

- Diagramas em **Mermaid** (o GitHub renderiza); árvore de pastas pode ficar em bloco de texto.
- Docs e specs em português.
- Padrão novo de Go que apareceu? Registre em [`docs/GO-LEARNINGS.md`](docs/GO-LEARNINGS.md), com o código real.
