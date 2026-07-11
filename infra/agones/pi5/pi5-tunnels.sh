#!/usr/bin/env bash
# pi5/pi5-tunnels.sh — UM único túnel ssh que encaminha para a interface
# do Mac as duas portas que o docker-compose do backend (go-multiplayer-control-plane)
# precisa alcançar no Pi:
#
#   36443 → 6443    API do k3s   (session-service aloca GameServers)
#   39090 → 32090   Prometheus   (federation do Prometheus do backend)
#
# Substitui os dois scripts manuais anteriores (start-pi-k8s-tunnel.sh e
# start-pi-prometheus-tunnel.sh):
#   - O Prometheus do Pi é NodePort 32090, então ssh -L alcança as DUAS portas
#     num processo só — mais robusto que `kubectl port-forward` (que cai em
#     idle, não reconecta e depende do kubeconfig).
#   - Por que túnel: Docker Desktop no Mac não roteia container → LAN; o compose
#     aponta para host.docker.internal:{36443,39090}.
#
# Uso:
#   pi5/pi5-tunnels.sh run        # foreground (Ctrl+C para parar)
#   pi5/pi5-tunnels.sh install    # LaunchAgent: sobe no login e revive
#   pi5/pi5-tunnels.sh uninstall  # remove o LaunchAgent
#   pi5/pi5-tunnels.sh status     # estado do agent + teste das 2 portas
#   pi5/pi5-tunnels.sh logs       # tail -f do log do agent
#
# Config por env (com defaults do homelab da Fase 7):
#   PI_HOST=vitor@192.168.0.34
#   K8S_LOCAL=36443  K8S_REMOTE=6443
#   PROM_LOCAL=39090 PROM_REMOTE=32090
set -euo pipefail

PI_HOST="${PI_HOST:-vitor@192.168.0.34}"
K8S_LOCAL="${K8S_LOCAL:-36443}";   K8S_REMOTE="${K8S_REMOTE:-6443}"
PROM_LOCAL="${PROM_LOCAL:-39090}"; PROM_REMOTE="${PROM_REMOTE:-32090}"

LABEL="com.packetarena.pi5-tunnels"
PLIST="${HOME}/Library/LaunchAgents/${LABEL}.plist"
LOG="${HOME}/Library/Logs/pi5-tunnels.log"
SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"

# --- foreground: o processo do túnel (é o que o LaunchAgent executa) ---------
run() {
  echo "Túnel Pi → Mac  |  k3s ${K8S_LOCAL}→${K8S_REMOTE}  ·  prom ${PROM_LOCAL}→${PROM_REMOTE}  |  ${PI_HOST}"
  echo "$(date '+%Y-%m-%d %H:%M:%S')  abrindo túnel ssh…"
  # -N              sem comando remoto (só forwarding)
  # BatchMode       nunca pede senha (exige chave) — falha rápido p/ o KeepAlive
  # ExitOnForward.. se a porta local não abrir, sai (o LaunchAgent reinicia)
  # ServerAlive*    detecta link morto (Wi-Fi/reboot do Pi) em ~45s e sai
  exec ssh -N \
    -o BatchMode=yes \
    -o ExitOnForwardFailure=yes \
    -o ConnectTimeout=10 \
    -o ServerAliveInterval=15 \
    -o ServerAliveCountMax=3 \
    -o StrictHostKeyChecking=accept-new \
    -L "0.0.0.0:${K8S_LOCAL}:127.0.0.1:${K8S_REMOTE}" \
    -L "0.0.0.0:${PROM_LOCAL}:127.0.0.1:${PROM_REMOTE}" \
    "${PI_HOST}"
}

# --- launchd: instala/carrega o agent ----------------------------------------
install() {
  mkdir -p "$(dirname "$PLIST")" "$(dirname "$LOG")"
  cat > "$PLIST" <<PLIST_EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>${LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>${SELF}</string>
    <string>run</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PI_HOST</key><string>${PI_HOST}</string>
    <key>K8S_LOCAL</key><string>${K8S_LOCAL}</string>
    <key>K8S_REMOTE</key><string>${K8S_REMOTE}</string>
    <key>PROM_LOCAL</key><string>${PROM_LOCAL}</string>
    <key>PROM_REMOTE</key><string>${PROM_REMOTE}</string>
    <key>PATH</key><string>/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/opt/homebrew/bin</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ThrottleInterval</key><integer>10</integer>
  <key>StandardOutPath</key><string>${LOG}</string>
  <key>StandardErrorPath</key><string>${LOG}</string>
</dict>
</plist>
PLIST_EOF
  # recarrega idempotente
  launchctl unload "$PLIST" 2>/dev/null || true
  launchctl load -w "$PLIST"
  echo "LaunchAgent instalado: ${PLIST}"
  echo "  PI_HOST=${PI_HOST}  |  k3s ${K8S_LOCAL}→${K8S_REMOTE}  ·  prom ${PROM_LOCAL}→${PROM_REMOTE}"
  echo "  log: ${LOG}"
  echo "Aguarde ~5s e rode:  ${SELF} status"
}

uninstall() {
  launchctl unload "$PLIST" 2>/dev/null || true
  rm -f "$PLIST"
  echo "LaunchAgent removido: ${PLIST}"
}

status() {
  echo "== LaunchAgent =="
  if launchctl list | grep -q "$LABEL"; then
    launchctl list | grep "$LABEL" | sed 's/^/  /'
    echo "  (colunas: PID  último-exit-status  Label — PID '-' = não rodando)"
  else
    echo "  não carregado (rode: ${SELF} install)"
  fi
  echo "== Portas locais =="
  local k p
  k=$(curl -sk -o /dev/null -w "%{http_code}" --max-time 5 "https://127.0.0.1:${K8S_LOCAL}/healthz" || echo "erro")
  p=$(curl -s  -o /dev/null -w "%{http_code}" --max-time 5 "http://127.0.0.1:${PROM_LOCAL}/-/healthy" || echo "erro")
  echo "  k3s  ${K8S_LOCAL}: HTTP ${k}   (401/200 = túnel ok · erro/000 = fora)"
  echo "  prom ${PROM_LOCAL}: HTTP ${p}   (200 = túnel ok · erro/000 = fora)"
}

logs() { touch "$LOG"; tail -n 40 -f "$LOG"; }

case "${1:-}" in
  run)       run ;;
  install)   install ;;
  uninstall) uninstall ;;
  status)    status ;;
  logs)      logs ;;
  *) echo "uso: $0 {run|install|uninstall|status|logs}" >&2; exit 2 ;;
esac
