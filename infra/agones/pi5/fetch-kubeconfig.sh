#!/usr/bin/env bash
# pi5/fetch-kubeconfig.sh — kubeconfigs do cluster do Pi para o Mac
#. Gera dois arquivos AQUI (ambos gitignored — credenciais):
#   kubeconfig-pi5.yaml         → uso pessoal (kubectl/helm do Mac)
#   kubeconfig-compose-pi5.yaml → mount do session-service (compose)
# Os dois apontam para https://<PI_IP>:6443 — diferente do k3d, o mesmo
# endereço vale de fora e de dentro do compose (é um IP real da LAN).
#
#   PI_HOST=vitor@192.168.x.x PI_IP=192.168.x.x pi5/fetch-kubeconfig.sh
set -euo pipefail

PI_HOST="${PI_HOST:?defina PI_HOST (usuário@host do Pi)}"
PI_IP="${PI_IP:?defina PI_IP (IP reservado do Pi na LAN)}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# /etc/rancher/k3s/k3s.yaml é root-only; cat com sudo evita mexer em permissão.
ssh "${PI_HOST}" "sudo cat /etc/rancher/k3s/k3s.yaml" \
  | sed "s/127\.0\.0\.1:6443/${PI_IP}:6443/" \
  > "${SCRIPT_DIR}/kubeconfig-pi5.yaml"
chmod 600 "${SCRIPT_DIR}/kubeconfig-pi5.yaml"

# Compose: Docker Desktop não roteia container → LAN, então o session-service
# passa pelo túnel ssh do start-pi-k8s-tunnel.sh (host.docker.internal:36443).
# tls-server-name mantém a validação do cert contra o SAN do IP do Pi.
sed "s|server: https://${PI_IP}:6443|server: https://host.docker.internal:36443|" \
  "${SCRIPT_DIR}/kubeconfig-pi5.yaml" \
  | awk -v ip="${PI_IP}" '{print} /host\.docker\.internal:36443/{print "    tls-server-name: " ip}' \
  > "${SCRIPT_DIR}/kubeconfig-compose-pi5.yaml"
chmod 600 "${SCRIPT_DIR}/kubeconfig-compose-pi5.yaml"

KUBECONFIG="${SCRIPT_DIR}/kubeconfig-pi5.yaml" kubectl get nodes
echo "OK: ${SCRIPT_DIR}/kubeconfig-pi5.yaml (+ compose) — rode pi5/install-agones.sh"
