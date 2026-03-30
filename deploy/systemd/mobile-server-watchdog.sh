#!/usr/bin/env bash
set -euo pipefail

WORKDIR="/home/wikki/deploy/mobile_server_deploy"
CORE_HEALTH_URL="${CORE_HEALTH_URL:-http://127.0.0.1:8081/healthz}"
PUBLIC_HEALTH_URL="${PUBLIC_HEALTH_URL:-https://core.wspace.sbs/healthz}"
LOCK_FILE="${LOCK_FILE:-/tmp/mobile-server-watchdog.lock}"
CHECK_TIMEOUT="${CHECK_TIMEOUT:-5}"

log() {
  printf '[mobile-server-watchdog] %s\n' "$1"
}

health_ok() {
  local url="$1"
  curl -fsS --max-time "$CHECK_TIMEOUT" "$url" >/dev/null 2>&1
}

restart_unit() {
  local unit="$1"
  log "restarting ${unit}"
  systemctl restart "$unit"
}

wait_for_health() {
  local url="$1"
  local label="$2"
  for _ in $(seq 1 10); do
    if health_ok "$url"; then
      log "${label} is healthy"
      return 0
    fi
    sleep 1
  done
  log "${label} is still unhealthy"
  return 1
}

mkdir -p "$(dirname "$LOCK_FILE")"
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  log "another watchdog run is already active"
  exit 0
fi

cd "$WORKDIR"

if ! health_ok "$CORE_HEALTH_URL"; then
  restart_unit "mobile-server-core.service"
  wait_for_health "$CORE_HEALTH_URL" "core" || exit 1
else
  log "core health ok"
fi

if ! pgrep -f 'cloudflared.*accord-vision-core' >/dev/null 2>&1; then
  restart_unit "mobile-server-tunnel.service"
  sleep 2
fi

if ! health_ok "$PUBLIC_HEALTH_URL"; then
  restart_unit "mobile-server-tunnel.service"
  wait_for_health "$PUBLIC_HEALTH_URL" "public tunnel" || exit 1
else
  log "public tunnel health ok"
fi
