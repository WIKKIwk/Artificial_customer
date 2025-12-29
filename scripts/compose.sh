#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-upg}"

ACTION="${1:-}"
shift || true

if [ -z "$ACTION" ]; then
  echo "Usage: scripts/compose.sh <up|down|ps> [args...]"
  exit 1
fi

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

ensure_env_files() {
  if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
      cp ".env.example" ".env"
    else
      : > ".env"
    fi
  fi

  if [ -d "database" ] && [ ! -f "database/.env" ] && [ -f "database/.env.example" ]; then
    cp "database/.env.example" "database/.env"
  fi
}

start_docker_service() {
  if has_cmd systemctl; then
    systemctl enable --now docker >/dev/null 2>&1 || true
    return
  fi
  if has_cmd rc-service; then
    rc-service docker start >/dev/null 2>&1 || true
    return
  fi
  if has_cmd service; then
    service docker start >/dev/null 2>&1 || true
  fi
}

install_docker_as_root() {
  if has_cmd apt-get; then
    apt-get update -y
    apt-get install -y docker.io docker-compose-plugin || apt-get install -y docker-compose
    return
  fi
  if has_cmd apk; then
    apk add --no-cache docker docker-compose
    return
  fi
  if has_cmd yum; then
    yum install -y docker docker-compose-plugin || yum install -y docker-compose
    return
  fi
  if has_cmd dnf; then
    dnf install -y docker docker-compose-plugin || dnf install -y docker-compose
    return
  fi
  if has_cmd pacman; then
    pacman -Sy --noconfirm docker docker-compose
    return
  fi
  if has_cmd emerge; then
    emerge --quiet-build --ask=n app-containers/docker app-containers/docker-compose || true
    return
  fi
  if has_cmd curl; then
    curl -fsSL https://get.docker.com | sh
    return
  fi
  echo "Docker o'rnatish uchun paket menejeri topilmadi."
  exit 1
}

install_docker() {
  if [ "${SKIP_DOCKER_INSTALL:-}" = "1" ]; then
    echo "Docker o'rnatish SKIP_DOCKER_INSTALL=1 sababli o'tkazildi."
    return
  fi
  if [ "$(id -u)" = "0" ]; then
    install_docker_as_root
    start_docker_service
    return
  fi
  if has_cmd sudo; then
    if has_cmd apt-get; then
      sudo apt-get update -y
      sudo apt-get install -y docker.io docker-compose-plugin || sudo apt-get install -y docker-compose
    elif has_cmd apk; then
      sudo apk add --no-cache docker docker-compose
    elif has_cmd yum; then
      sudo yum install -y docker docker-compose-plugin || sudo yum install -y docker-compose
    elif has_cmd dnf; then
      sudo dnf install -y docker docker-compose-plugin || sudo dnf install -y docker-compose
    elif has_cmd pacman; then
      sudo pacman -Sy --noconfirm docker docker-compose
    elif has_cmd emerge; then
      sudo emerge --quiet-build --ask=n app-containers/docker app-containers/docker-compose || true
    elif has_cmd curl; then
      sudo sh -c 'curl -fsSL https://get.docker.com | sh'
    else
      echo "Docker o'rnatish uchun paket menejeri topilmadi."
      exit 1
    fi
    start_docker_service
    return
  fi
  echo "Docker o'rnatish uchun root/sudo kerak."
  exit 1
}

compose_cmd=""
compose_prefix=""

detect_compose_cmd() {
  compose_cmd=""
  compose_prefix=""

  if has_cmd docker; then
    if docker compose version >/dev/null 2>&1; then
      compose_cmd="docker compose"
      return
    fi
    if has_cmd sudo && sudo -n docker compose version >/dev/null 2>&1; then
      compose_cmd="docker compose"
      compose_prefix="sudo "
      return
    fi
  fi

  if has_cmd docker-compose; then
    compose_cmd="docker-compose"
    return
  fi
  if has_cmd sudo && sudo -n docker-compose version >/dev/null 2>&1; then
    compose_cmd="docker-compose"
    compose_prefix="sudo "
    return
  fi
}

ensure_env_files
detect_compose_cmd

if [ -z "$compose_cmd" ]; then
  echo "Docker compose topilmadi. O'rnatishga harakat qilinmoqda..."
  install_docker
  detect_compose_cmd
fi

if [ -z "$compose_cmd" ]; then
  echo "Docker compose hali ham topilmadi."
  exit 1
fi

if has_cmd docker; then
  if ! docker info >/dev/null 2>&1; then
    start_docker_service
    if ! docker info >/dev/null 2>&1; then
      if [ -z "$compose_prefix" ] && has_cmd sudo && sudo -n docker info >/dev/null 2>&1; then
        compose_prefix="sudo "
      else
        echo "Docker daemon ishlamayapti yoki ruxsat yetarli emas."
        exit 1
      fi
    fi
  fi
fi

if [ -n "$compose_prefix" ]; then
  compose_cmd="${compose_prefix}${compose_cmd}"
fi

COMPOSE_FILES="-f docker-compose.yml -f docker-compose.database.yml"
COMPOSE_BAKE=false $compose_cmd $COMPOSE_FILES "$ACTION" "$@"
