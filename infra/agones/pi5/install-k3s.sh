#!/usr/bin/env bash
# pi5/install-k3s.sh — k3s single-node no Pi. Roda DO MAC,
# executa no Pi via ssh. Idempotente (o instalador do k3s faz upgrade in-place).
#
# Sem Traefik/ServiceLB: GameServer usa hostPort direto, não há ingress.
# TLS SAN do IP: o kubectl do Mac e o session-service (compose) validam o
# certificado do API server acessando https://<PI_IP>:6443.
#
#   PI_HOST=vitor@192.168.x.x PI_IP=192.168.x.x pi5/install-k3s.sh
set -euo pipefail

PI_HOST="${PI_HOST:?defina PI_HOST (usuário@host do Pi)}"
PI_IP="${PI_IP:?defina PI_IP (IP reservado do Pi na LAN)}"

ssh "${PI_HOST}" "curl -sfL https://get.k3s.io | \
  INSTALL_K3S_EXEC='server --disable traefik --disable servicelb --tls-san ${PI_IP}' sh -"

# systemd sobe o k3s no boot (reboot test da spec); aqui só o aceite imediato.
ssh "${PI_HOST}" "sudo k3s kubectl get nodes"
echo "OK: k3s instalado — rode pi5/fetch-kubeconfig.sh"
