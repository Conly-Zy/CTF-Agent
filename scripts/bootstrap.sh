#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

USE_BUILD=0
NO_START=0

usage() {
  cat <<'USAGE'
Usage: scripts/bootstrap.sh [--build] [--no-start] [-h|--help]

Fresh-environment bootstrap for CTF-Agent.

Options:
  --build      Build the Docker image locally before starting.
  --no-start   Only create .env/config/data directories; do not start Docker Compose.
  -h, --help   Show this help.

Common examples:
  ANTHROPIC_API_KEY="sk-..." ./scripts/bootstrap.sh
  LLM_PROVIDER=openai OPENAI_API_KEY="sk-..." LLM_MODEL="gpt-4.1" ./scripts/bootstrap.sh
  ./scripts/bootstrap.sh --build
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build)
      USE_BUILD=1
      shift
      ;;
    --no-start)
      NO_START=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

upsert_env() {
  local key="$1"
  local value="$2"
  local file=".env"
  local tmp
  tmp="$(mktemp)"

  if [[ -f "$file" ]] && grep -q "^${key}=" "$file"; then
    awk -v k="$key" -v v="$value" 'BEGIN { replaced=0 } $0 ~ "^" k "=" { print k "=" v; replaced=1; next } { print } END { if (!replaced) print k "=" v }' "$file" > "$tmp"
  else
    [[ -f "$file" ]] && cat "$file" > "$tmp"
    printf '%s=%s\n' "$key" "$value" >> "$tmp"
  fi

  mv "$tmp" "$file"
}

read_env_value() {
  local key="$1"
  if [[ -f .env ]]; then
    grep -E "^${key}=" .env | tail -n 1 | cut -d= -f2- || true
  fi
}

need_cmd docker
if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose plugin is required. Install Docker Desktop or docker compose v2." >&2
  exit 1
fi

mkdir -p data uploads logs backups plugins

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "Created .env from .env.example"
fi

if [[ ! -f data/config.yaml ]]; then
  cp config.yaml.example data/config.yaml
  echo "Created data/config.yaml from config.yaml.example"
fi

# Carry through environment-provided settings into .env so future compose runs are reproducible.
for key in CTF_PORT TZ CTF_AGENT_IMAGE CTF_AGENT_IMAGE_TAG LLM_PROVIDER LLM_MODEL LLM_BASE_URL ANTHROPIC_API_KEY OPENAI_API_KEY LLM_API_KEY; do
  if [[ -n "${!key:-}" ]]; then
    upsert_env "$key" "${!key}"
  fi
done

# If no key is configured, optionally prompt in interactive terminals. The server can still start without a key,
# but solving tasks will fail until a provider key is supplied.
configured_key="$(read_env_value ANTHROPIC_API_KEY)$(read_env_value OPENAI_API_KEY)$(read_env_value LLM_API_KEY)"
if [[ -z "$configured_key" && -t 0 ]]; then
  provider="$(read_env_value LLM_PROVIDER)"
  provider="${provider:-anthropic}"
  if [[ "$provider" == "openai" ]]; then
    printf 'Enter OPENAI_API_KEY (leave empty to skip): '
    IFS= read -r -s key || true
    printf '\n'
    [[ -n "${key:-}" ]] && upsert_env OPENAI_API_KEY "$key"
  else
    printf 'Enter ANTHROPIC_API_KEY (leave empty to skip): '
    IFS= read -r -s key || true
    printf '\n'
    [[ -n "${key:-}" ]] && upsert_env ANTHROPIC_API_KEY "$key"
  fi
fi

if [[ "$NO_START" -eq 1 ]]; then
  echo "Bootstrap files are ready. Edit .env/data/config.yaml, then run docker compose up -d."
  exit 0
fi

compose_args=(-f docker-compose.yml)
if [[ "$USE_BUILD" -eq 1 ]]; then
  compose_args+=(-f docker-compose.build.yml)
  echo "Building and starting CTF-Agent locally..."
  docker compose "${compose_args[@]}" up -d --build
else
  echo "Pulling and starting CTF-Agent from GHCR..."
  docker compose "${compose_args[@]}" pull ctf-agent || true
  docker compose "${compose_args[@]}" up -d
fi

port="$(read_env_value CTF_PORT)"
port="${port:-4399}"
url="http://localhost:${port}"

if command -v curl >/dev/null 2>&1; then
  for _ in $(seq 1 30); do
    if curl -fsS "${url}/api/health/live" >/dev/null 2>&1; then
      echo "CTF-Agent is ready: ${url}"
      exit 0
    fi
    sleep 1
  done
fi

echo "CTF-Agent is starting: ${url}"
echo "Check logs with: docker compose logs -f ctf-agent"
