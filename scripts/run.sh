#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_SCRIPT="sh ./scripts/compose.sh"

timestamp="$(date +%Y%m%d-%H%M%S)"
log_dir="${UPG_RUN_LOG_DIR:-/tmp}"
log_file="${UPG_RUN_LOG_FILE:-$log_dir/upg-run-$timestamp.log}"

use_color=false
if [ -t 1 ] && command -v tput >/dev/null 2>&1; then
  if tput colors >/dev/null 2>&1; then
    use_color=true
  fi
fi

if [ "$use_color" = "true" ]; then
  BOLD="$(tput bold)"
  DIM="$(tput dim)"
  GREEN="$(tput setaf 2)"
  RED="$(tput setaf 1)"
  YELLOW="$(tput setaf 3)"
  CYAN="$(tput setaf 6)"
  RESET="$(tput sgr0)"
  HIDE_CURSOR="$(tput civis || true)"
  SHOW_CURSOR="$(tput cnorm || true)"
else
  BOLD=""
  DIM=""
  GREEN=""
  RED=""
  YELLOW=""
  CYAN=""
  RESET=""
  HIDE_CURSOR=""
  SHOW_CURSOR=""
fi

cleanup() {
  printf "%s" "$SHOW_CURSOR" >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM

spinner_step() {
  # $1 = pid, $2 = label
  pid="$1"
  label="$2"
  i=0
  printf "%s" "$HIDE_CURSOR" >/dev/null 2>&1 || true
  while kill -0 "$pid" >/dev/null 2>&1; do
    i=$((i + 1))
    case $((i % 4)) in
      0) c='|' ;;
      1) c='/' ;;
      2) c='-' ;;
      3) c='\\' ;;
    esac
    printf "\r%s %s" "$c" "$label"
    sleep 0.12
  done
  printf "\r%s %s\n" "${GREEN}✓${RESET}" "$label"
}

wait_for_url() {
  # $1 = url, $2 = label, $3 = timeout seconds
  url="$1"
  label="$2"
  timeout="${3:-60}"
  start="$(date +%s)"
  i=0

  printf "%s" "$HIDE_CURSOR" >/dev/null 2>&1 || true
  while :; do
    if curl -fsS --max-time 1 "$url" >/dev/null 2>&1; then
      printf "\r%s %s\n" "${GREEN}✓${RESET}" "$label"
      return 0
    fi

    now="$(date +%s)"
    if [ $((now - start)) -ge "$timeout" ]; then
      printf "\r%s %s\n" "${RED}✗${RESET}" "$label"
      return 1
    fi

    i=$((i + 1))
    case $((i % 4)) in
      0) c='|' ;;
      1) c='/' ;;
      2) c='-' ;;
      3) c='\\' ;;
    esac
    printf "\r%s %s" "$c" "$label"
    sleep 0.2
  done
}

printf "\n%sUPG%s starting...\n" "$BOLD" "$RESET"
printf "%sLog:%s %s\n\n" "$DIM" "$RESET" "$log_file"

mkdir -p "$log_dir" 2>/dev/null || true

(
  $COMPOSE_SCRIPT up -d --build --force-recreate --remove-orphans
) >"$log_file" 2>&1 &
compose_pid="$!"

spinner_step "$compose_pid" "Docker Compose (build + up)"

if ! wait "$compose_pid"; then
  printf "\n%sStart failed.%s Logs:\n" "$RED" "$RESET"
  tail -n 200 "$log_file" || true
  exit 1
fi

printf "\n%sChecking services...%s\n" "$BOLD" "$RESET"
ok=true

wait_for_url "http://localhost:8080/health" "API ready (http://localhost:8080/health)" 60 || ok=false
wait_for_url "http://localhost:8001/" "UI ready  (http://localhost:8001)" 60 || ok=false

if [ "$ok" != "true" ]; then
  printf "\n%sSome services did not become ready.%s\n" "$YELLOW" "$RESET"
  printf "%sTip:%s run 'make logs' or check %s\n\n" "$DIM" "$RESET" "$log_file"
  exit 1
fi

printf "\n%sReady:%s\n" "$BOLD" "$RESET"
printf "  %sUI:%s  http://localhost:8001\n" "$CYAN" "$RESET"
printf "  %sAPI:%s http://localhost:8080\n" "$CYAN" "$RESET"
printf "  %sWS:%s  ws://localhost:4000/socket\n" "$CYAN" "$RESET"
printf "\n%sCommands:%s make logs | make ps | make stop\n\n" "$DIM" "$RESET"
