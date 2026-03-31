#!/usr/bin/env bash
set -euo pipefail

WORKDIR="/home/wikki/deploy/mobile_server_deploy"
CORE_HEALTH_URL="${CORE_HEALTH_URL:-http://127.0.0.1:8081/healthz}"
CORE_LOGIN_URL="${CORE_LOGIN_URL:-http://127.0.0.1:8081/v1/mobile/auth/login}"
WERKA_SUMMARY_URL="${WERKA_SUMMARY_URL:-http://127.0.0.1:8081/v1/mobile/werka/summary}"
PUBLIC_HEALTH_URL="${PUBLIC_HEALTH_URL:-https://core.wspace.sbs/healthz}"
LOCK_FILE="${LOCK_FILE:-/tmp/mobile-server-watchdog.lock}"
CHECK_TIMEOUT="${CHECK_TIMEOUT:-5}"
DEEP_CHECK_TIMEOUT="${DEEP_CHECK_TIMEOUT:-20}"

log() {
  printf '[mobile-server-watchdog] %s\n' "$1"
}

health_ok() {
  local url="$1"
  curl -fsS --max-time "$CHECK_TIMEOUT" "$url" >/dev/null 2>&1
}

werka_summary_ok() {
  if [ -z "${WERKA_PHONE:-}" ] || [ -z "${MOBILE_DEV_WERKA_CODE:-}" ]; then
    log "werka creds missing; skipping deep summary check"
    return 0
  fi
  local payload
  payload=$(printf '{"phone":"%s","code":"%s"}' "$WERKA_PHONE" "$MOBILE_DEV_WERKA_CODE")
  local login_body
  login_body="$(curl -fsS --max-time "$DEEP_CHECK_TIMEOUT" -H 'Content-Type: application/json' -d "$payload" "$CORE_LOGIN_URL")" || return 1
  local token
  token="$(printf '%s' "$login_body" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token",""))')" || return 1
  [ -n "$token" ] || return 1
  curl -fsS --max-time "$DEEP_CHECK_TIMEOUT" -H "Authorization: Bearer $token" "$WERKA_SUMMARY_URL" >/dev/null 2>&1
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

wait_for_werka_summary() {
  for _ in $(seq 1 10); do
    if werka_summary_ok; then
      log "werka summary is healthy"
      return 0
    fi
    sleep 2
  done
  log "werka summary is still unhealthy"
  return 1
}

mkdir -p "$(dirname "$LOCK_FILE")"
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  log "another watchdog run is already active"
  exit 0
fi

cd "$WORKDIR"
[ -f ./.env ] && . ./.env

if ! health_ok "$CORE_HEALTH_URL" || ! werka_summary_ok; then
  restart_unit "mobile-server-core.service"
  wait_for_health "$CORE_HEALTH_URL" "core" || exit 1
  wait_for_werka_summary || exit 1
else
  log "core and werka summary health ok"
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
