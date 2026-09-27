#!/usr/bin/env bash
# infra/docker/gen-secrets.sh — gera os segredos de dev do compose (gitignored).
# Idempotente: não sobrescreve chave existente (sobrescrever invalidaria os
# JWKS já baixados pelos game servers Ready).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mkdir -p "${SCRIPT_DIR}/secrets"

# session-jwt: connect token (JWKS baixado pelos game servers).
# identity-jwt: token do jogador (JWKS lido pelos serviços player-facing).
for name in session-jwt identity-jwt; do
  KEY="${SCRIPT_DIR}/secrets/${name}-ed25519.pem"
  if [[ -f "${KEY}" ]]; then
    echo "já existe: ${KEY} (mantido)"
  else
    openssl genpkey -algorithm ed25519 -out "${KEY}"
    chmod 600 "${KEY}"
    echo "gerado: ${KEY}"
  fi
done
