#!/usr/bin/env bash
# stage.sh — os "atores" dos vídeos de feature: jogadores de mentira que movem
# o console enquanto a gravação roda (mesmas chamadas do scripts/demo-host-world.sh).
#
#   stage.sh world GAME_ID [MODE_ID]   host abre um mundo p2p, avisa (heartbeat) e um invasor entra
#   stage.sh kill                      o jogo do host reporta HOST_KILLED do último mundo aberto
#   stage.sh stop                      para o heartbeat do último mundo
#   stage.sh seed                      dá inventário, stats e um save na nuvem ao invasor do último mundo
#
# O estado do último mundo fica em $STATE (padrão /tmp/stage-world.env).
set -euo pipefail

IDENTITY="${IDENTITY:-http://localhost:8081}"
SESSION_GRPC="${SESSION_GRPC:-localhost:50052}"
MM_GRPC="${MM_GRPC:-localhost:50051}"
PLAYER_DATA="${PLAYER_DATA:-http://localhost:8083}"
CLOUD_SAVE="${CLOUD_SAVE:-http://localhost:8084}"
STATE="${STATE:-/tmp/stage-world.env}"

login() { # login HANDLE GAME_ID -> token
  curl -sS -o /dev/null -X POST "$IDENTITY/accounts" -d "{\"handle\":\"$1\",\"password\":\"supersecret\"}"
  curl -sS -X POST "$IDENTITY/sessions" -d "{\"handle\":\"$1\",\"password\":\"supersecret\",\"gameId\":\"$2\"}" | jq -r .token
}
sub() { local p; p="$(cut -d. -f2 <<<"$1" | tr '_-' '/+')"; while (( ${#p} % 4 )); do p+="="; done; base64 -d <<<"$p" | jq -r .sub; }

case "${1:-}" in
world)
  game_id="$2"; mode="${3:-}"
  n=$((RANDOM % 90 + 10))
  host_token="$(login "caipora_$n" "$game_id")"
  invader_token="$(login "matinta_$n" "$game_id")"; invader_id="$(sub "$invader_token")"
  conn="steam:7656119800$((RANDOM % 9000000 + 1000000))"
  # nível do host e do invasor: o modo pode exigir faixa de nível (matchmaking/012)
  req="{\"game_id\":\"$game_id\",\"host_connection\":\"$conn\",\"host_soul_level\":40${mode:+,\"mode_id\":\"$mode\"}}"
  world="$(grpcurl -plaintext -H "authorization: Bearer $host_token" -d "$req" "$SESSION_GRPC" session.v1.Session/OpenWorld)"
  session_id="$(jq -r .sessionId <<<"$world")"; world_token="$(jq -r .worldToken <<<"$world")"
  hb() { grpcurl -plaintext -H "x-world-token: $world_token" -d "{\"session_id\":\"$session_id\"}" "$SESSION_GRPC" session.v1.Session/Heartbeat >/dev/null; }
  hb
  # heartbeat a cada 10 s enquanto o vídeo roda (o TTL é 90 s)
  ( while sleep 10; do hb || exit 0; done ) >/dev/null 2>&1 &
  echo "session_id=$session_id world_token=$world_token invader_id=$invader_id invader_handle=matinta_$n hb_pid=$! game_id=$game_id" > "$STATE"
  sleep "${INVADE_AFTER:-4}"
  grpcurl -plaintext -H "authorization: Bearer $invader_token" -d "{\"game_id\":\"$game_id\",\"soul_level\":38${mode:+,\"mode_id\":\"$mode\"}}" \
    "$MM_GRPC" matchmaking.v1.Matchmaking/Invade >/dev/null
  echo "mundo $session_id aberto por caipora_$n; matinta_$n invadiu"
  ;;
kill)
  # shellcheck disable=SC1090
  source <(tr ' ' '\n' < "$STATE")
  grpcurl -plaintext -H "x-world-token: $world_token" \
    -d "{\"session_id\":\"$session_id\",\"player_id\":\"$invader_id\",\"role\":\"invader\",\"event\":\"HOST_KILLED\",\"outcome\":\"SUCCESS\",\"event_id\":\"stage-$session_id\"}" \
    "$SESSION_GRPC" session.v1.Session/ReportParticipationEvent >/dev/null
  echo "HOST_KILLED reportado em $session_id"
  ;;
seed)
  source <(tr ' ' '\n' < "$STATE")
  tok="$(login "$invader_handle" "$game_id")"
  put() { curl -sS -f -o /dev/null -X PUT "$1" -H "Authorization: Bearer $tok" -H "X-Game-Id: $game_id" -H "Content-Type: application/json" -d "$2"; }
  put "$PLAYER_DATA/me/data/inventario/mochila" '{"ouro":1280,"itens":[{"id":"faca_de_ticum","qtd":1},{"id":"cachaca_benta","qtd":3},{"id":"pena_de_uirapuru","qtd":2}]}'
  put "$PLAYER_DATA/me/data/progresso/stats" '{"nivel":38,"invasoes_vencidas":1,"bosses_derrotados":["boitata"],"horas_jogadas":12.5}'
  put "$CLOUD_SAVE/me/save" '{"slot":1,"regiao":"Clareira do Rio","checkpoint":"fogueira_do_rio","nivel":38,"equipado":["faca_de_ticum","capa_de_palha"]}'
  echo "dados e save de $invader_handle ($invader_id)"
  ;;
stop)
  source <(tr ' ' '\n' < "$STATE")
  kill "$hb_pid" 2>/dev/null || true
  ;;
*) sed -n 2,9p "$0"; exit 2 ;;
esac
