#!/usr/bin/env bash
# infra/agones/apply-fleet.sh — aplica infra/agones/fleets/<fleet-name>.yaml no
# cluster Agones ativo (ADR-0029: um script só para todas as Fleets, em vez de
# um por repo de data plane). O nome da fleet é o mesmo valor cadastrado via
# `PUT /games/{id}/fleet` (ADR-0026).
#
# Dois modos, pelo mesmo manifest:
#   - k3d local: sem MAC_HOST — aplica direto no contexto ativo do kubectl
#     (o próprio k3d resolve host.k3d.internal).
#   - Pi 5 (homelab): MAC_HOST=<ip-do-mac> — usa pi5/kubeconfig-pi5.yaml e
#     substitui host.k3d.internal pelo IP real do Mac na LAN (sem essa
#     resolução automática fora do Docker Desktop).
#
#   infra/agones/apply-fleet.sh packet-arena
#   MAC_HOST=192.168.x.y infra/agones/apply-fleet.sh packet-arena
set -euo pipefail

FLEET_NAME="${1:?uso: apply-fleet.sh <fleet-name> (ex.: packet-arena)}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FLEET="${SCRIPT_DIR}/fleets/${FLEET_NAME}.yaml"

[ -f "${FLEET}" ] || { echo "fleet não encontrada: ${FLEET}" >&2; exit 1; }

if [ -n "${MAC_HOST:-}" ]; then
  export KUBECONFIG="${SCRIPT_DIR}/pi5/kubeconfig-pi5.yaml"
  sed "s/host\.k3d\.internal/${MAC_HOST}/g" "${FLEET}" | kubectl apply -f -
  echo "OK: ${FLEET_NAME} aplicada no Pi com backend em ${MAC_HOST}"
else
  kubectl apply -f "${FLEET}"
  echo "OK: ${FLEET_NAME} aplicada no cluster ativo do kubectl (k3d local)"
fi
kubectl get gs
