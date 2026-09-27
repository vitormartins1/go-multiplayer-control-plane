## O que muda

<!-- Um resumo curto e a spec que este PR implementa (services/<serviço>/specs/NNN-slug). -->

## Como foi validado

<!-- Testes, execução local, prints do console. -->

- [ ] `go test ./... -race` e `golangci-lint run ./...`
- [ ] testes de integração, se tocou repositório ou mensageria (`go test -tags=integration ./...`)
- [ ] `npm run lint && npm test && npm run build`, se tocou o console

## Checklist

- [ ] spec criada ou atualizada, com as tasks marcadas
- [ ] contrato `.proto` só com mudanças aditivas, registradas em `docs/contracts/CHANGELOG.md`
- [ ] decisão arquitetural nova registrada como ADR
- [ ] documentação atualizada

## Fora deste PR

<!-- O que ficou de fora de propósito. -->
