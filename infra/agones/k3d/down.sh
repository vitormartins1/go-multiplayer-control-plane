#!/usr/bin/env bash
# tools/k3d/down.sh — destrói o cluster local (descartável; up.sh recria).
set -euo pipefail

CLUSTER="${CLUSTER:-packet-arena}"
k3d cluster delete "${CLUSTER}"
