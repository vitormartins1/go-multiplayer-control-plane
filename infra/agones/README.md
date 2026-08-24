# infra/agones — cluster Agones do control plane (ADR-0029)

Bootstrap de cluster + Fleet manifests, centralizados aqui em vez de duplicados
em cada `unity-<archetype>-data-plane` (ADR-0029, Fatia 1). Fica **fora** deste
repo apenas o que constrói a imagem do dedicated server (`build-server.sh`,
`import-image.sh`, `push-image.sh`) — isso é específico do código-fonte/tooling
de cada data plane Unity.

Dois ambientes, mesmo Fleet manifest (`fleets/<nome>.yaml`), mesmo
`apply-fleet.sh`:

## k3d (cluster local, descartável)

```sh
infra/agones/k3d/up.sh              # cria o cluster + instala o Agones (idempotente)
# em cada data plane: ./import-image.sh (importa a imagem recém-buildada)
infra/agones/apply-fleet.sh packet-arena
kubectl get gs -w                   # Ready → Allocated → Shutdown → novo Ready
infra/agones/k3d/down.sh            # destrói tudo
```

Decisões de config (`k3d/up.sh`): portas UDP 7100–7139 mapeadas do node k3d
para o host (client no Mac conecta em `127.0.0.1:<porta>`); API server na porta
6550 com SAN extra `host.docker.internal` (o session-service em Docker Compose
alcança o cluster de dentro do container). `up.sh` gera
`k3d/kubeconfig-compose.yaml` (gitignored — credenciais do cluster local), que
o compose do backend monta quando `ALLOCATOR_MODE=agones`
(`infra/docker/docker-compose.agones.yml`).

### Monitoramento (dashboard "Local (k3d)" no Grafana do compose)

```sh
infra/agones/k3d/install-monitoring.sh   # kube-prometheus-stack, sem Grafana (Prometheus NodePort 32090 → host 39091)
```

Mesmo espírito do Pi 5 (ver abaixo): sem Grafana local — o Prometheus do
compose (Mac) federa este Prometheus via `job_name: local-federation`
(`infra/docker/prometheus.yml`, alvo `host.docker.internal:39091`, label
`cluster: local`). Dashboard: `infra/docker/grafana/provisioning/dashboards/local-agones-fleet.json`.
O NodePort→host (`39091:32090@server:0`) já é criado pelo `up.sh` em clusters
novos; num cluster k3d já existente sem essa porta, `up.sh` adiciona via
`k3d cluster edit --port-add` (idempotente, não recria o cluster nem perde
Fleets/GameServers já aplicados).

**Cuidado com nomes de Fleet iguais nos dois clusters:** k3d e Pi 5 costumam ter
Fleets com o mesmo nome (`packet-arena`, `folclore-souls` — ex-`capoeira-souls`) — toda query dos
dois dashboards filtra por `cluster="local"`/`cluster="pi5"` de propósito; uma
query nova sem esse filtro soma os dois clusters junto, silenciosamente.

## Pi 5 (homelab dedicado)

```sh
export PI_HOST=vitor@192.168.x.x   # usuário@IP do Pi
export PI_IP=192.168.x.x
export MAC_HOST=192.168.x.y        # IP reservado do Mac

infra/agones/pi5/install-k3s.sh        # k3s single-node no Pi (via ssh)
infra/agones/pi5/fetch-kubeconfig.sh   # gera pi5/kubeconfig-pi5.yaml (+ compose)
infra/agones/pi5/install-agones.sh     # Agones 1.58.0 via helm (range 7100-7139)
# em cada data plane: ./push-image.sh (docker save | ssh | k3s ctr images import)
MAC_HOST=${MAC_HOST} infra/agones/apply-fleet.sh packet-arena
infra/agones/pi5/install-monitoring.sh # Prometheus (sem Grafana — dashboard "Pi 5" vive no Grafana do compose, ver abaixo)
```

Aceites parciais:

```sh
kubectl --kubeconfig infra/agones/pi5/kubeconfig-pi5.yaml get nodes   # Pi Ready
kubectl --kubeconfig infra/agones/pi5/kubeconfig-pi5.yaml get gs      # N Ready
```

### Backend aloca no Pi (compose no Mac)

```sh
docker compose -f docker-compose.yml -f docker-compose.agones-pi5.yml up -d session-service
```

O override monta `pi5/kubeconfig-compose-pi5.yaml` e **não define
`DEV_PUBLIC_HOST`** — o client conecta no IP real do Pi. Voltar ao k3d = subir
com o override `docker-compose.agones.yml` de novo.

### Túnel Pi → Mac (`pi5-tunnels.sh`) — suba ANTES do compose

Docker Desktop no Mac não roteia container → LAN, então o compose do backend
alcança o Pi por `host.docker.internal:<porta>` — essas portas são abertas por
um túnel ssh no Mac (k3s API + Prometheus, num único processo):

```sh
infra/agones/pi5/pi5-tunnels.sh install    # LaunchAgent: sobe no login e revive se cair
infra/agones/pi5/pi5-tunnels.sh status     # estado do agent + testa as 2 portas
infra/agones/pi5/pi5-tunnels.sh logs       # tail -f do log
infra/agones/pi5/pi5-tunnels.sh uninstall  # remove o LaunchAgent
```

Exige chave ssh sem prompt (BatchMode) — se a chave tem passphrase, deixe-a no
ssh-agent. Mudou o IP do Pi (sem reserva DHCP)?
`PI_HOST=vitor@<novo-ip> pi5-tunnels.sh install`.

## Fleets (`fleets/<nome>.yaml`)

Um manifest por Fleet, nome = o mesmo valor cadastrado via
`PUT /games/{id}/fleet` (ADR-0026). `apply-fleet.sh <nome>` aplica o manifest
certo em qualquer um dos dois ambientes acima — ver o script para o
mecanismo de substituição do host do backend (`host.k3d.internal` → `MAC_HOST`
no modo Pi).

## Notas

- **Grafana é sempre o do compose** (`http://localhost:3000`, serviço `grafana`
  em `infra/docker/docker-compose.yml`) — nem k3d nem Pi 5 rodam Grafana
  próprio (`grafana.enabled: false` nos dois `monitoring-values.yaml`); cada
  cluster só roda Prometheus (+ node-exporter/kube-state-metrics), federado
  pelo Prometheus do compose. Dois dashboards: `pi5-agones-fleet.json` e
  `local-agones-fleet.json` (`infra/docker/grafana/provisioning/dashboards/`).
- `kubeconfig-*.yaml` em `k3d/` e `pi5/` são **gitignored** (credenciais do
  cluster).
- O IP do Mac fica gravado na Fleet aplicada no Pi — mudou o IP, rode
  `apply-fleet.sh` de novo (por isso a reserva DHCP).
- `k3s ctr images import` (nos scripts `push-image.sh` de cada data plane) usa
  o containerd do k3s (não docker) e exige sudo.
- Fatia 2 da ADR-0029 (registry de imagem por jogo, CI publicando no GHCR) é
  **gated por evidência** — não implementada.
