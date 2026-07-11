# Spec 023 — status real da Fleet Agones (cluster + health)

**Serviço:** session-service (fatia cross-service: admin-bff, admin-ui)
**Status:** Draft
**Data:** 2026-07-11
**Relaciona:** ADR-0026 (fleet por jogo), ADR-0029 (infra Agones centralizada), admin-ui/specs/059-fleet-tab (Fase C do "Visão de evolução futura" daquela spec — hoje só documentada, não aprovada)
**Depende de:** nada novo no game-registry (o campo `fleet` por jogo já é exposto de ponta a ponta desde a spec 059 do admin-ui).

## Contexto

O cluster do Pi 5 hoje roda **duas** Fleets Agones (`packet-arena`, `capoeira-souls`) — e o k3d local também, quando o allocator `agones` está ativo lá. A aba Fleet do admin-ui (`admin-ui/specs/059-fleet-tab`) já existe, mas a seção de "imagem/health/versões" é **inteiramente mockada** (dado ilustrativo determinístico, sem chamada de rede) — a spec 059 documentou essa evolução como 4 fases futuras (A: CI publica imagem; B: lista tags do registry; **C: lê estado real do Agones**; D: botão Promover de verdade) e deixou todas fora de escopo, gated por decisão própria.

Esta spec implementa a **Fase C** (e só ela): não imagem/versão (ainda não existe registry, Fase A/B), não o botão Promover (Fase D) — só a pergunta operacional imediata que motivou a revisão: **onde essa Fleet está rodando agora, e está saudável?** Motivação do Vitor: um sinal visual análogo ao que a aba Infrastructure já mostra pros serviços Go (badge saudável/fora/desconhecido, `StatusBadge`/`lib/status.ts`) — aqui adaptado pra 3 estados de **localização** (não just up/down): `offline` (não observada em nenhum cluster que o backend ativo alcança), `local` (cluster k3d do Mac) ou `pi5` (cluster do Raspberry Pi), mais os números reais de réplicas (desired/ready/allocated) já que estamos tocando na aba mesmo.

**Limitação arquitetural aceita nesta spec** (documentada, não resolvida): o session-service só se conecta a **um** cluster por vez — o kubeconfig é escolhido no boot via qual override do `docker-compose.yml` está ativo (`agones.yml`=k3d, `agones-pi5.yml`=Pi 5, nenhum=allocator `static`/sem Agones). Não existe hoje (nem esta spec cria) um jeito de o backend consultar os DOIS clusters simultaneamente. Então "offline" aqui significa precisamente "não encontrada no cluster que o backend ativo está observando agora" — pode, na prática, estar rodando no outro cluster sem que este backend saiba. Aceitável para o cenário de homelab de um único desenvolvedor (ver Fora de escopo).

## Comportamento (EARS)

1. **O SISTEMA DEVE** ler uma nova env `AGONES_CLUSTER_LABEL` no boot do session-service (string livre, ex. `"local"`/`"pi5"`; ausente/vazia quando o allocator não é `agones`) — setada nos overrides de compose existentes (`docker-compose.agones.yml` → `local`; `docker-compose.agones-pi5.yml` → `pi5`), sem exigir nenhum arquivo/env novo além disso.
2. **QUANDO** o admin consulta o status de uma Fleet pelo nome **E** o allocator ativo não é `agones` (é `static`), **O SISTEMA DEVE** devolver `cluster=""`, `fleetExists=false`, réplicas zeradas — sem tentar nenhuma chamada k8s.
3. **QUANDO** o allocator ativo é `agones` **E** a Fleet com o nome pedido não existe no cluster observado (`NotFound` do k8s), **O SISTEMA DEVE** devolver `cluster=<AGONES_CLUSTER_LABEL>`, `fleetExists=false`, réplicas zeradas — distinto do caso 2 (aqui SABEMOS que não está *neste* cluster; lá não temos cluster nenhum pra checar).
4. **QUANDO** o allocator ativo é `agones` **E** a Fleet existe, **O SISTEMA DEVE** devolver `cluster=<AGONES_CLUSTER_LABEL>`, `fleetExists=true`, e os campos `desired` (`spec.replicas`), `current` (`status.replicas`), `ready` (`status.readyReplicas`), `allocated` (`status.allocatedReplicas`) lidos do recurso `fleets.agones.dev` no momento da chamada (sem cache).
5. **O SISTEMA DEVE** expor esse comportamento via `GET /admin/fleet-status/{fleetName}` no session-service (mesma superfície `/admin` sem auth-na-borda de `spec 005`/`018`, mesmo padrão de `AdminHandler`).
6. **admin-bff DEVE** expor `GET /games/{gameId}/fleet-status`, resolvendo `gameId → fleet name` (já lido do game-registry, spec 059) e repassando pro endpoint acima; **SE** o jogo não tiver `fleet` cadastrada, devolve o equivalente ao caso 2 sem chamar o session-service; **SE** o session-service estiver inalcançável, devolve um erro que o admin-ui trata como estado "desconhecido" (não crasha a página).
7. **admin-ui DEVE** mostrar, na linha/card já expandido de cada jogo com fleet cadastrada (`Fleet.tsx`, spec 059), uma nova seção **real** "Onde está rodando" com um badge de 3 estados (`offline`/`local`/`pi5`/`desconhecido` — 4 visualmente, "desconhecido" só pra falha de rede) + os números reais de réplicas quando `fleetExists=true` — **sem** remover a seção mockada de imagem/versão (essa continua Fase A/B/D, fora de escopo).
8. **O SISTEMA NÃO DEVE** cachear o resultado entre chamadas — cada abertura/expansão da linha da Fleet dispara uma leitura fresca (baixo volume, uso interno de console).

## Modelo de Dados

```go
// session-service: internal/domain
type FleetStatus struct {
    Cluster     string // "", "local", "pi5" (ou outro valor livre de AGONES_CLUSTER_LABEL)
    FleetExists bool
    Desired     int
    Current     int
    Ready       int
    Allocated   int
}
```

JSON (`GET /admin/fleet-status/{fleetName}` e `GET /games/{gameId}/fleet-status`, mesmo shape):
```json
{"cluster": "pi5", "fleetExists": true, "desired": 2, "current": 2, "ready": 2, "allocated": 0}
```

## Casos de Teste Obrigatórios

- [ ] Allocator `static` → `cluster=""`, `fleetExists=false`, réplicas zeradas, nenhuma chamada k8s (mock/spy no client)
- [ ] Allocator `agones`, Fleet inexistente (nome errado) → `cluster=<label>`, `fleetExists=false`
- [ ] Allocator `agones`, Fleet existente → todos os campos numéricos corretos (fixture/fake k8s client com um Fleet de teste)
- [ ] admin-bff: jogo sem `fleet` cadastrada → não chama session-service, devolve o shape "offline" direto
- [ ] admin-bff: session-service fora do ar → erro tratável, não propaga panic/500 cru
- [ ] admin-ui: 3 estados renderizados corretamente (`fleetExists=false`+`cluster=""` → "offline"; `cluster="local"` → "local (mac)"; `cluster="pi5"` → "pi 5"); erro de rede → "desconhecido"
- [ ] `go test ./services/{session-service,admin-bff}/... -race` limpo; `tsc --noEmit` + `npm run build` no admin-ui limpos

## Fora de escopo

- **Fases A, B, D** da "Visão de evolução futura" da spec 059 (imagem/build publicado, listagem de tags via GHCR, botão Promover de verdade) — continuam mock, intocadas por esta spec.
- **Observar os dois clusters simultaneamente** — arquitetura de um-cluster-por-instância aceita como está (ver Contexto); não cria um segundo client k8s nem um serviço agregador.
- **Histórico/série temporal no admin-ui** — isso já existe no Grafana (`infra/agones/pi5/agones-grafana-dashboard.yaml`, quebrado por `fleet_name` nesta mesma rodada de trabalho); aqui é só snapshot do momento.
- **Contagem de `Unhealthy`** — o recurso `Fleet` do Agones não expõe esse campo em `status` (só o `GameServer` individual tem `state=Unhealthy`, já coberto pelo dashboard Grafana); não teria como preencher sem uma segunda chamada (LIST de GameServers) que esta spec não julga necessária pro objetivo (localização + saúde agregada básica).
