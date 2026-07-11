#!/usr/bin/env bash
# k3d/install-monitoring.sh — Prometheus no cluster k3d local (espelha
# pi5/install-monitoring.sh). Stack: kube-prometheus-stack (node-exporter,
# kube-state-metrics, Prometheus — sem Grafana, que fica no compose do Mac).
#
# Pré-requisito: up.sh já rodou (cria o NodePort 32090 → host 39091, ver
# MONITORING_PORT/MONITORING_NODEPORT em up.sh). Sem o mapeamento de porta, o
# job local-federation do compose (infra/docker/prometheus.yml) não alcança.
#
#   infra/agones/k3d/install-monitoring.sh
set -euo pipefail

CLUSTER="${CLUSTER:-packet-arena}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export KUBECONFIG="${SCRIPT_DIR}/kubeconfig-k3d.yaml"

k3d kubeconfig write "${CLUSTER}" --output "${KUBECONFIG}" --overwrite >/dev/null

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null
helm repo update prometheus-community >/dev/null

helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace \
  -f "${SCRIPT_DIR}/monitoring-values.yaml" \
  --wait --timeout 15m

echo ""
echo "OK: monitoramento instalado no k3d"
echo "    Prometheus (local, debug): kubectl --kubeconfig ${KUBECONFIG} port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9091:9090"
echo "    Federado pelo compose em host.docker.internal:39091 (job local-federation)"
echo "    Dashboard: infra/docker/grafana/provisioning/dashboards/local-agones-fleet.json"
