#!/usr/bin/env bash
# tools/k3d/up.sh — cluster k3d local + Agones (spec 005-agones).
#
# Cria o cluster `packet-arena` com o range de portas UDP dos GameServers
# mapeado para o host (client no Mac alcança o pod) e instala o Agones via
# Helm com o range estreitado para casar com o mapeamento. Idempotente:
# roda de novo sem destruir o cluster (helm upgrade --install).
#
# Pré-requisitos: k3d, helm, kubectl (brew install k3d helm kubectl).
set -euo pipefail

CLUSTER="${CLUSTER:-packet-arena}"
AGONES_CHART_VERSION="${AGONES_CHART_VERSION:-1.58.0}"
# Range estreito: mapear 7000-8000 inteiro criaria milhares de proxies no Docker.
MIN_PORT=7100
MAX_PORT=7139
API_PORT=6550
# NodePort do Prometheus (install-monitoring.sh) — federado pelo Prometheus do
# compose (job local-federation, infra/docker/prometheus.yml) em
# host.docker.internal:39091. Mesmo papel do 32090 do Pi 5.
MONITORING_PORT=39091
MONITORING_NODEPORT=32090

if ! k3d cluster list -o json | grep -q "\"name\": *\"${CLUSTER}\""; then
  # --tls-san: o session-service (docker compose) alcança o API server via
  # host.docker.internal; sem o SAN a verificação TLS do kubeconfig falharia.
  k3d cluster create "${CLUSTER}" \
    --api-port "${API_PORT}" \
    --port "${MIN_PORT}-${MAX_PORT}:${MIN_PORT}-${MAX_PORT}/udp@server:0" \
    --port "${MONITORING_PORT}:${MONITORING_NODEPORT}@server:0" \
    --k3s-arg "--tls-san=host.docker.internal@server:*"
else
  echo "cluster ${CLUSTER} já existe — seguindo para o Agones"
  # Cluster criado antes do NodePort de monitoramento existir nesta versão do
  # script: adiciona o mapeamento sem recriar (k3d cluster edit, experimental
  # mas idempotente — repetir não duplica). Silencioso se já existir.
  k3d cluster edit "${CLUSTER}" --port-add "${MONITORING_PORT}:${MONITORING_NODEPORT}@server:0" 2>/dev/null || true
fi

helm repo add agones https://agones.dev/chart/stable >/dev/null
helm repo update agones >/dev/null

helm upgrade --install agones agones/agones \
  --namespace agones-system --create-namespace \
  --version "${AGONES_CHART_VERSION}" \
  --set gameservers.minPort="${MIN_PORT}" \
  --set gameservers.maxPort="${MAX_PORT}" \
  --wait --timeout 5m

kubectl get ns agones-system

# Kubeconfig para o session-service (compose): host reescrito para
# host.docker.internal. Contém credenciais do cluster local — gitignored.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
k3d kubeconfig get "${CLUSTER}" \
  | sed "s/0\.0\.0\.0:${API_PORT}/host.docker.internal:${API_PORT}/" \
  > "${SCRIPT_DIR}/kubeconfig-compose.yaml"
echo "kubeconfig para o compose: ${SCRIPT_DIR}/kubeconfig-compose.yaml"
