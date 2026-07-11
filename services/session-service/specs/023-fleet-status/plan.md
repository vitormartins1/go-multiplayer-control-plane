# Plan: 023-fleet-status
**Spec:** specs/023-fleet-status/spec.md
**Data:** 2026-07-11

## Abordagem técnica

Reaproveitar o adapter Agones já existente (`internal/allocator/agones.go`) — ele já tem o `dynamic.Interface` e o `namespace` prontos, só falta um GVR novo (`fleets.agones.dev`) e um método de leitura. Nova Port `domain.FleetInspector` (opcional, mesmo espírito do `ServerRegistry`: só o adapter Agones implementa; o adapter `static` não existe pra Fleet nenhuma). `AGONES_CLUSTER_LABEL` é uma env simples lida no `main.go`, guardada junto do `AdminHandler` — não precisa virar parte do adapter (é metadado do deployment, não do cluster em si).

Cadeia de chamada: `admin-ui → admin-bff (GET /games/{id}/fleet-status) → session-service (GET /admin/fleet-status/{fleet}) → k8s dynamic client (fleets.agones.dev)`.

## Arquivos a criar ou modificar

**session-service:**
- `internal/domain/session.go` — novo `type FleetStatus struct {...}` + `type FleetInspector interface { FleetStatus(ctx, fleetName string) (FleetStatus, error) }`
- `internal/allocator/agones.go` — `fleetGVR = schema.GroupVersionResource{Group: "agones.dev", Version: "v1", Resource: "fleets"}`; método `FleetStatus(ctx, fleetName string) (domain.FleetStatus, error)` (Get by name; `k8serrors.IsNotFound` → `FleetExists: false`, sem erro; ler `spec.replicas`/`status.{replicas,readyReplicas,allocatedReplicas}` via `unstructured.NestedInt64`)
- `internal/transport/http_handler.go` — `AdminHandler` ganha campo `fleets domain.FleetInspector` (pode ser `nil`, mesmo padrão do `registry ServerRegistry` opcional) + `clusterLabel string`; rota nova `GET /admin/fleet-status/{fleetName}`; handler monta o JSON combinando `clusterLabel` (sempre, mesmo com `fleets == nil`) com o resultado de `FleetStatus` (ou zerado se `fleets == nil`, allocator não é agones)
- `cmd/server/main.go` — lê `AGONES_CLUSTER_LABEL` (`os.Getenv`, default `""`); passa o adapter Agones (já construído) como `FleetInspector` pro `NewAdminHandler` só quando o allocator ativo é o Agones (mesmo `if` que já decide static vs agones hoje)
- `infra/docker/docker-compose.agones.yml` — adiciona `AGONES_CLUSTER_LABEL: local` no `session-service.environment`
- `infra/docker/docker-compose.agones-pi5.yml` — adiciona `AGONES_CLUSTER_LABEL: pi5`

**admin-bff:**
- `internal/domain/admin.go` (ou arquivo de fleet dedicado) — `type FleetStatus struct { Cluster string; FleetExists bool; Desired, Current, Ready, Allocated int }` + Port `FleetStatusReader interface { FleetStatus(ctx, gameID string) (FleetStatus, error) }`
- `internal/session/client.go` — método novo `FleetStatus(ctx, fleetName string) (domain.FleetStatus, error)` fazendo `GET {baseURL}/admin/fleet-status/{fleetName}` (mesmo padrão de `getSessions`/`CloseSession`)
- `internal/service/admin_service.go` — método que resolve `gameID → fleet` (já tem o `Game` do game-registry client) e chama o client de sessão; jogo sem fleet cadastrada devolve o shape "offline" sem chamar session-service (EARS item 6)
- `internal/transport/http_handler.go` — rota `GET /games/{gameId}/fleet-status`

**admin-ui:**
- `src/lib/api.ts` — tipo `FleetStatus` + função `api.fleetStatus(gameId, signal)`
- `src/lib/fleetLocation.ts` (novo, mesmo espírito de `lib/status.ts`) — `type FleetLocation = "offline" | "local" | "pi5" | "unknown"`; `fleetLocationColor`/`fleetLocationLabel` (ex.: `local` → azul "local (mac)"; `pi5` → verde "pi 5"; `offline` → cinza "offline"; `unknown` → cinza "desconhecido")
- `src/components/FleetLocationBadge.tsx` (novo, espelha `StatusBadge.tsx`)
- `src/pages/Fleet.tsx` — `FleetRow` ganha `useAsync` (ou hook equivalente) pro `api.fleetStatus`, disparado quando a linha expande (não no mount da lista inteira — evita N chamadas simultâneas ao abrir a aba); nova seção com o badge + réplicas reais, ao lado (não substituindo) da seção mockada existente

## Compute e banco (ADR-0005)

N/A — nenhuma mudança de compute/banco; reaproveita o k8s dynamic client já provisionado no session-service.

## Dependências e libs

Nenhuma nova — `k8s.io/apimachinery`/`k8s.io/client-go` já são dependências do `internal/allocator`.

## Decisões técnicas e trade-offs

- **Cluster identity via env var simples (`AGONES_CLUSTER_LABEL`), não derivada do kubeconfig.** Alternativa considerada: inferir do nome do context/cluster no kubeconfig montado (`k3d-packet-arena` vs `default`/nome do node `packet-arena-pi`). Rejeitada: acopla a um detalhe de nomenclatura do kubeconfig que já mudou uma vez (ADR-0029) e pode mudar de novo; a env var é uma linha nos dois `docker-compose.agones*.yml` que já existem e já são o lugar certo pra declarar "qual ambiente é este".
- **Fleet como Port opcional (`nil`-ável), não interface obrigatória.** Mesma decisão já tomada pro `ServerRegistry` (spec 008) — o adapter `static` não tem Fleet nenhuma pra inspecionar; forçar uma implementação vazia seria ruído. O handler HTTP já sabe lidar com Port opcional ausente (mesmo padrão condicional do `main.go` atual).
- **Sem cache/TTL no resultado.** Volume de chamadas é baixo (só quando o operador expande uma linha da aba Fleet no console) — um cache adicionaria staleness sem ganho de performance perceptível. Se o padrão de uso mudar (ex.: polling automático no futuro), revisar.
- **Não resolve observar os dois clusters ao mesmo tempo** — ver "Limitação arquitetural aceita" na spec. Não é uma decisão técnica desta spec, é um limite herdado da arquitetura atual (um allocator configurado por processo); resolver isso é fora de escopo e provavelmente nem vale a pena pro cenário de homelab de um dev só.

## Ordem de implementação

1. session-service: domain (`FleetStatus`/`FleetInspector`) → allocator (`FleetStatus` no adapter Agones, com teste usando fake dynamic client) → transport (rota + handler, com teste) → main.go (wiring + env) → compose overrides (env)
2. admin-bff: domain → session client (`FleetStatus`, com teste) → service (resolução gameID→fleet, com teste) → transport (rota, com teste)
3. admin-ui: `lib/fleetLocation.ts` → `FleetLocationBadge` → `lib/api.ts` → `Fleet.tsx` (integração)
4. Verificação end-to-end: `docker compose ... up` com override `agones` (k3d) e depois `agones-pi5`, confirmar que a aba Fleet mostra `local`/`pi5` corretamente pros jogos com fleet aplicada e "offline" pros que não têm

## Riscos e pontos de atenção

- **Nome do recurso `Fleet` no Agones é `metadata.name`, igual ao campo `fleet` do jogo** — já é a convenção estabelecida (ADR-0026); nenhuma tradução nova necessária, só confirmar que o `Get` usa o nome exato sem normalização extra.
- **`k8serrors.IsNotFound` no `dynamic.Interface.Get`** — confirmar que o comportamento é idêntico ao já usado em `Release` (spec 004) antes de assumir que funciona igual pra `fleets.agones.dev` (GVR diferente, mas mesmo client/mecanismo — risco baixo).
- **admin-bff chamando session-service por HTTP dentro do request do admin-ui** — latência adicional (rede local, k8s API do outro lado) na primeira expansão de cada linha; aceitável pro volume de uso (console interno), mas vale um timeout explícito curto (ex. 3s) pra não travar a UI se o session-service ou o k8s estiverem lentos.
