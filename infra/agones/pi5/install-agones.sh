#!/usr/bin/env bash
# pi5/install-agones.sh — Agones no k3s do Pi. Roda no Mac
# com o kubeconfig gerado pelo fetch-kubeconfig.sh. Mesmo chart/range da
# Fase 5 (tools/k3d/up.sh): 1.58.0 pinado, GameServers em UDP 7100-7139
# (num node real subir o range é só config aqui + firewall, se houver).
# Imagens do Agones são multi-arch — cobrem o node ARM64. Idempotente.
set -euo pipefail

AGONES_CHART_VERSION="${AGONES_CHART_VERSION:-1.58.0}"
MIN_PORT=7100
MAX_PORT=7139
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export KUBECONFIG="${SCRIPT_DIR}/kubeconfig-pi5.yaml"

helm repo add agones https://agones.dev/chart/stable >/dev/null
helm repo update agones >/dev/null

helm upgrade --install agones agones/agones \
  --namespace agones-system --create-namespace \
  --version "${AGONES_CHART_VERSION}" \
  --set gameservers.minPort="${MIN_PORT}" \
  --set gameservers.maxPort="${MAX_PORT}" \
  --wait --timeout 10m

kubectl get ns agones-system
echo "OK: Agones ${AGONES_CHART_VERSION} — rode pi5/push-image.sh e apply-fleet.sh"
