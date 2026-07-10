# Plan 022 — allocator fleet-aware

## Decisão de design: onde mora a checagem de "fleet vazio + Agones = erro"

O `Service` só enxerga `domain.ServerAllocator` (interface) — não sabe em runtime se o adapter ativo é `Agones` ou `static`. Em vez de um marker interface novo, a checagem entra **dentro do próprio adapter Agones** (mesmo padrão já documentado ali: "matchID/gameID viram annotations... o static ignora"): `Allocate` ganha um parâmetro `fleet string`; o `Agones.Allocate` recusa com `ErrFleetUnavailable` quando `fleet == ""`; o `static.Allocate` recebe o parâmetro e o ignora (nunca erra por causa dele), como já faz com `gameID`. `Service.Create`/`OpenWorld` só repassam `snap.Fleet` — sem branch novo, sem checagem prévia.

## Domain (`internal/domain/session.go` + `errors.go`)

- `ProfileSnapshot` (linha 342-353) ganha `Fleet string`.
- `ServerAllocator.Allocate` (linha 378-383) muda de `(ctx, matchID, gameID string)` para `(ctx, matchID, gameID, fleet string)` — doc do método atualizada citando o novo comportamento (Agones recusa vazio, static ignora).
- `errors.go`: novo `ErrFleetUnavailable = errors.New("session: fleet not configured")`, ao lado de `ErrProfileUnavailable`.

## Adapter Agones (`internal/allocator/agones.go`)

- `NewAgones(client dynamic.Interface, namespace string) *Agones` — remove o parâmetro/campo `fleet` da struct e do construtor (linha 31-41).
- `Allocate(ctx, matchID, gameID, fleet string)`: primeira linha do corpo — `if fleet == "" { return domain.Server{}, fmt.Errorf("agones: fleet not set: %w", domain.ErrFleetUnavailable) }`. O selector (linha 54-55) troca `a.fleet` por `fleet` (o parâmetro).

## Adapter static (`internal/allocator/static.go` ou onde estiver)

- `Allocate` ganha o parâmetro `fleet string` na assinatura (satisfaz a interface); corpo não muda — mesmo tratamento que já dá a `gameID` (recebido, não usado na lógica de pool).

## Registry client (`internal/registryclient/client.go`)

- `modeResponse` (linha 38-42) ganha `Fleet string \`json:"fleet"\`` — o game-registry (spec 012, item 5) já embute isso na resposta de `GET .../multiplayer-profiles/{modeId}`.
- `ModeProfile`: `snap.Fleet = m.Fleet` na montagem do `domain.ProfileSnapshot` (perto da linha 83-89). `profileHash` **não** inclui `Fleet` no hash (o hash é de {archetype, topology, params} — atributo de infra, não de configuração de gameplay que o data plane valida via manifest; manter o hash estável evita invalidar snapshots existentes por um campo que o Unity nem lê).

## Service (`internal/service/session_service.go`)

- `Create` (linha 128-142): `s.allocator.Allocate(ctx, matchID, gameID, snap.Fleet)` — só troca o call-site, sem lógica nova.
- `OpenWorld` (linha 569-586): idem, `s.allocator.Allocate(ctx, sessionID, gameID, snap.Fleet)`.
- Nenhuma outra mudança — o erro `ErrFleetUnavailable` propaga naturalmente pelo `%w` já existente ("...: allocate: %w").

## Transport gRPC (`internal/transport/grpc_server.go`)

- `CreateSession` (switch linha ~40-49): novo `case errors.Is(err, domain.ErrFleetUnavailable): return nil, status.Error(codes.FailedPrecondition, "fleet not configured")`.
- `worldError` (linha ~182-199, usado por `OpenWorld` e demais chamadas host_world): mesmo `case` novo.

## `cmd/server/main.go`

- Remove a leitura de `AGONES_FLEET` (linha 241) e o argumento correspondente em `NewAgones` (linha 243) — vira `allocator.NewAgones(client, namespace)`.

## Rollout (fora do código, ver spec)

- Antes do deploy: `PUT /games/{queue-arena-game-id}/fleet {"fleet":"packet-arena"}` no game-registry (spec 012), senão a 1ª `CreateSession` depois do deploy falha com `FailedPrecondition`.

## Testes

- `internal/allocator/agones_test.go`: `Allocate` com `fleet=""` → `ErrFleetUnavailable`, sem chamar a API k8s; `Allocate` com fleet não-vazio → selector usa o valor recebido (não mais um campo fixo); duas chamadas com fleets diferentes → selectors diferentes.
- `internal/allocator/static_test.go` (se existir): `Allocate` ignora `fleet` (vazio ou não, sucesso igual).
- `internal/registryclient/client_test.go`: resposta HTTP com `"fleet":"packet-arena"` → `snap.Fleet == "packet-arena"`; resposta sem o campo → `snap.Fleet == ""` (compat com game-registry pré-spec-012, não deveria quebrar).
- `internal/service/session_service_test.go`: `Create`/`OpenWorld` passam `snap.Fleet` para `Allocate` (mock `AssertCalled` com o valor esperado); snapshot com `Fleet=""` e mock do allocator devolvendo `ErrFleetUnavailable` → erro propaga, sessão não é criada, `issuer.Issue` não é chamado.
- `internal/transport/grpc_server_test.go` / `host_world_grpc_test.go`: `ErrFleetUnavailable` → `codes.FailedPrecondition` em `CreateSession` e `OpenWorld`.
- Suite completa (`go test ./... -race -cover`) verde.
