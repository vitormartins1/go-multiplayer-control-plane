# Plan: 003-admin-read
**Spec:** specs/003-admin-read/spec.md
**Data:** 2026-06-29

## Abordagem técnica

Feature **só de transport** (ADR-0002): a lógica já existe (`AchievementService.ListForPlayer` e `ListCatalog`, das features 001/002). Adicionamos duas rotas `/admin/...` **fora** do middleware `auth` (são control plane interno, sem JWT no MVP) que resolvem o `playerID` do **path** via `r.PathValue` (roteador do `net/http`, Go 1.22). Os handlers reusam os DTOs e helpers já existentes (`toPlayerAchievementDTOs`, `toAchievementDTOs`). Nenhuma mudança em `domain`, `service` ou `repository`.

## Arquivos a criar ou modificar

- `internal/transport/http_handler.go` —
  - `Register`: adicionar `GET /admin/players/{playerID}/achievements` e `GET /admin/achievements`, **sem** o wrapper `h.auth`
  - `adminListForPlayer`: lê `r.PathValue("playerID")` e chama `h.svc.ListForPlayer(ctx, playerID)`; reusa `toPlayerAchievementDTOs`
  - `adminListCatalog`: chama `h.svc.ListCatalog(ctx)`; reusa `toAchievementDTOs` (idêntico ao `listCatalog`, sem auth)
- `internal/transport/http_handler_test.go` — testes dos novos endpoints (mock do `AchievementService`, `httptest`)
- `cmd/server/main.go` — **sem mudança** (já chama `handler.Register(mux)`)
- compose / migrations / infra — **sem mudança** (mesmo serviço, mesma porta, mesmo banco)

## Compute e banco (ver ADR-0005)
- Sem mudança: mesmo serviço, mesmo Postgres do achievements. Esta feature não toca dados, só expõe leitura existente por outra rota.

## Dependências e libs
- Nenhuma nova. `net/http` (`r.PathValue`), `encoding/json` (já usados).

## Decisões técnicas e trade-offs
- **`/admin/...` sem auth no MVP** (decisão da spec, confirmada): a barreira é a rede (não exposto publicamente). Auth real (service-to-service) é evolução — provável ADR futuro, junto do fan-out do session e da auth do `admin-bff`.
- **Reusar `ListForPlayer`/`ListCatalog`** em vez de novos métodos: o admin precisa exatamente do mesmo dado do `/me`, só que para um `playerID` arbitrário — a diferença é **só de onde vem o id** (path vs JWT), então é mudança de transport, não de domínio.
- **Prefixo `/admin`** isola as rotas internas das públicas e deixa explícito o caráter administrativo (e facilita um futuro corte por auth/rede/gateway).

## Ordem de implementação
1. Adicionar as duas rotas + handlers em `http_handler.go`
2. Testes (`httptest` + mock do service): player com/sem progresso; catálogo; ausência de `Authorization` **não** bloqueia
3. `go test ./... -race` + `go build ./...`

## Riscos e pontos de atenção
- **Sem auth** é intencional, mas precisa ficar claro que esses endpoints **não** podem ser expostos publicamente — documentar no `api/README` do achievements.
- Garantir que o `/admin/...` realmente fica **fora** do `h.auth` (senão exigiria JWT e o BFF não conseguiria chamar).
- `r.PathValue` exige o pattern com `{playerID}` no método/rota (Go 1.22) — já é o padrão do projeto.
