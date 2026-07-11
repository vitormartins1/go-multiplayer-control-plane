#!/usr/bin/env bash
# pi5/install-monitoring.sh — Prometheus no cluster do Pi (ADR-0029). Stack:
# kube-prometheus-stack (node-exporter, kube-state-metrics, Prometheus — SEM
# Grafana, ver monitoring-values.yaml: os dashboards vivem no Grafana do
# compose no Mac, que federa este Prometheus via NodePort 32090). Agones
# scraped por endpoints (pods individuais — só o líder HA exporta as
# métricas de estado).
#
# Dashboard real: infra/docker/grafana/provisioning/dashboards/pi5-agones-fleet.json
# (Grafana do compose, http://localhost:3000) — não um ConfigMap neste
# cluster (agones-grafana-dashboard.yaml aqui é vestígio de antes da decisão
# de desligar o Grafana do Pi; não é lido por nada hoje).
#
#   PI_IP=192.168.x.x pi5/install-monitoring.sh
set -euo pipefail

PI_IP="${PI_IP:?defina PI_IP (IP do Pi na LAN)}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export KUBECONFIG="${SCRIPT_DIR}/kubeconfig-pi5.yaml"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null
helm repo update prometheus-community >/dev/null

helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace \
  -f "${SCRIPT_DIR}/monitoring-values.yaml" \
  --wait --timeout 15m

# Dashboard do Agones via ConfigMap (sidecar do Grafana detecta pelo label)
kubectl apply -f "${SCRIPT_DIR}/agones-grafana-dashboard.yaml"

echo ""
echo "OK: monitoramento instalado"
echo "    Grafana → http://${PI_IP}:32000  (admin / packet-arena)"
echo "    Dashboards: Node Exporter Full · Kubernetes · Agones Fleet"
