#!/usr/bin/env bash
# demo-host-world.sh — o fluxo do Folclore Souls (arquétipo host_world_dropin)
# ponta a ponta contra a stack local, sem Unity e sem Agones:
#
#   1. cadastra um jogo e o modo host_world_dropin (game-registry)
#   2. cadastra a conquista "Invasor Vitorioso" (achievements; a regra global
#      invader + HOST_KILLED + SUCCESS -> invasion_won já vem no seed)
#   3. cria host e invasor e loga os dois (identity, JWT EdDSA)
#   4. o host abre o mundo (session.OpenWorld -> aloca server + connect token)
#   5. o invasor invade (matchmaking.Invade, autenticado pelo bearer)
#   6. o "servidor do mundo" reporta HOST_KILLED (session.ReportParticipationEvent)
#   7. o fan-out SNS -> SQS entrega ao achievements-consumer e a conquista
#      aparece desbloqueada para o invasor
#
# Requisitos: stack no ar (docker compose -f infra/docker/docker-compose.yml up),
# curl, jq e grpcurl (go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest).
# As portas podem ser trocadas por env (IDENTITY, REGISTRY, ACHIEVEMENTS, SESSION_GRPC, MM_GRPC).
set -euo pipefail

IDENTITY="${IDENTITY:-http://localhost:8081}"
REGISTRY="${REGISTRY:-http://localhost:8090}"
ACHIEVEMENTS="${ACHIEVEMENTS:-http://localhost:8082}"
SESSION_GRPC="${SESSION_GRPC:-localhost:50052}"
MM_GRPC="${MM_GRPC:-localhost:50051}"
KEY="${INTERNAL_API_KEY:-dev-internal-key}"

for bin in curl jq grpcurl; do
  command -v "$bin" >/dev/null || { echo "falta: $bin" >&2; exit 1; }
done

run="$(date +%s)"
step() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
api() { # api METHOD URL [BODY] — falha com a resposta se não for 2xx
  local out code
  out="$(curl -sS -X "$1" "$2" -H "X-Internal-Key: $KEY" -H 'Content-Type: application/json' ${3:+-d "$3"} -w '\n%{http_code}')"
  code="${out##*$'\n'}"; out="${out%$'\n'*}"
  [[ "$code" == 2* ]] || { echo "HTTP $code em $1 $2: $out" >&2; exit 1; }
  printf '%s' "$out"
}

step "1. jogo + modo host_world_dropin"
game_id="$(api POST "$REGISTRY/games" "{\"slug\":\"demo-souls-$run\",\"name\":\"Demo Souls $run\"}" | jq -r .id)"
api PUT "$REGISTRY/games/$game_id/multiplayer-profiles/default" '{
  "displayName": "Mundo aberto",
  "archetype": "host_world_dropin", "topology": "dedicated",
  "params": {"enabledRoles": ["coop","invader"], "coopSlots": 2, "invaderSlots": 1,
             "invasionEnabled": true, "levelRangeEnabled": false, "weaponTierEnabled": false,
             "passwordEnabled": false}
}' >/dev/null
echo "game_id=$game_id"

step "2. conquista ligada ao event type invasion_won"
api POST "$ACHIEVEMENTS/admin/games/$game_id/achievements" \
  '{"id":"invasor_vitorioso","name":"Invasor Vitorioso","description":"Derrotou o host do mundo invadido","eventType":"invasion_won","target":1}' >/dev/null
echo "ok"

step "3. host e invasor (identity)"
login() { # login HANDLE -> token
  curl -sS -o /dev/null -X POST "$IDENTITY/accounts" -d "{\"handle\":\"$1\",\"password\":\"supersecret\"}"
  curl -sS -X POST "$IDENTITY/sessions" -d "{\"handle\":\"$1\",\"password\":\"supersecret\",\"gameId\":\"$game_id\"}" | jq -r .token
}
sub() { # claim sub do JWT (payload base64url sem padding)
  local p; p="$(cut -d. -f2 <<<"$1" | tr '_-' '/+')"
  while (( ${#p} % 4 )); do p+="="; done
  base64 -d <<<"$p" | jq -r .sub
}
host_token="$(login "host_$run")";       host_id="$(sub "$host_token")"
invader_token="$(login "invasor_$run")"; invader_id="$(sub "$invader_token")"
echo "host=$host_id invasor=$invader_id"

step "4. host abre o mundo (session.OpenWorld)"
world="$(grpcurl -plaintext -d "{\"host_player_id\":\"$host_id\",\"game_id\":\"$game_id\"}" "$SESSION_GRPC" session.v1.Session/OpenWorld)"
session_id="$(jq -r .sessionId <<<"$world")"
echo "mundo=$session_id server=$(jq -r .serverAddress <<<"$world")"

step "5. invasor invade (matchmaking.Invade, bearer do invasor)"
invaded="$(grpcurl -plaintext -H "authorization: Bearer $invader_token" -d "{\"game_id\":\"$game_id\"}" "$MM_GRPC" matchmaking.v1.Matchmaking/Invade)"
[[ "$(jq -r .sessionId <<<"$invaded")" == "$session_id" ]] || { echo "invadiu outro mundo: $invaded" >&2; exit 1; }
echo "entrou no mundo $session_id com connect token role=invader"

step "6. servidor do mundo reporta HOST_KILLED"
grpcurl -plaintext -d "{\"session_id\":\"$session_id\",\"player_id\":\"$invader_id\",\"role\":\"invader\",\"event\":\"HOST_KILLED\",\"outcome\":\"SUCCESS\",\"event_id\":\"demo-$run\"}" \
  "$SESSION_GRPC" session.v1.Session/ReportParticipationEvent >/dev/null
echo "mundo encerrado (host_killed); evento publicado no fan-out"

step "7. conquista desbloqueada para o invasor (fan-out assíncrono)"
for _ in $(seq 1 20); do
  unlocked="$(curl -sS "$ACHIEVEMENTS/me/achievements" -H "Authorization: Bearer $invader_token" -H "X-Game-Id: $game_id" \
    | jq -r '.achievements[] | select(.id=="invasor_vitorioso") | .unlocked')"
  [[ "$unlocked" == "true" ]] && { echo "✅ Invasor Vitorioso desbloqueada"; exit 0; }
  sleep 1
done
echo "❌ conquista não desbloqueou em 20s (fan-out/consumer no ar?)" >&2
exit 1
