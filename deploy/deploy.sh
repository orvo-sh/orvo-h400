#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [ ! -f .env.production ]; then
  echo ".env.production is required" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
. ./.env.production
set +a

mkdir -p runtime /opt/orvo/shared
touch runtime/collector.env

if [ -z "${ORVO_DOMAIN:-}" ] || [ -z "${CHAT_DOMAIN:-}" ]; then
  echo "ORVO_DOMAIN and CHAT_DOMAIN must be set for the Caddy reverse proxy" >&2
  exit 1
fi

bash deploy/install-docker.sh

export DOCKER_GID="$(stat -c '%g' /var/run/docker.sock)"

if docker compose config >/dev/null; then
  :
fi

docker compose build sandbox-image orvo
docker run --rm "${SANDBOX_DEFAULT_IMAGE:-orvo-opencode-sandbox:local}" \
  sh -lc 'git ls-remote https://github.com/octocat/Hello-World >/dev/null'
docker compose up -d postgres orvo

host_port="${ORVO_HTTP_PORT:-80}"
base_url="http://127.0.0.1:${host_port}"
for _ in $(seq 1 60); do
  if curl -fsS "${base_url}/api/v1/health/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

if ! curl -fsS "${base_url}/api/v1/health/ready" >/dev/null 2>&1; then
  echo "orvo API never became ready" >&2
  exit 1
fi

if [ -f runtime/bootstrap.env ]; then
  set -a
  # shellcheck disable=SC1091
  . runtime/bootstrap.env
  set +a
fi

BASE_URL="$base_url" \
SHARED_TELEMETRY_ENV_FILE="/opt/orvo/shared/orvo-telemetry.env" \
RUNTIME_COLLECTOR_ENV_FILE="runtime/collector.env" \
bash deploy/bootstrap-orvo.sh

host_identifier="$(hostname)"
tmp_collector_env="$(mktemp)"
grep -v '^HOST_IDENTIFIER=' runtime/collector.env >"$tmp_collector_env" || true
printf 'HOST_IDENTIFIER=%s\n' "$host_identifier" >>"$tmp_collector_env"
mv "$tmp_collector_env" runtime/collector.env

docker compose up -d otel-collector
docker compose up -d --force-recreate caddy

for _ in $(seq 1 20); do
  caddy_status="$(docker inspect -f '{{.State.Status}}' orvo-caddy 2>/dev/null || true)"
  if [ "$caddy_status" = "running" ]; then
    break
  fi
  sleep 2
done

if [ "$(docker inspect -f '{{.State.Status}}' orvo-caddy 2>/dev/null || true)" != "running" ]; then
  echo "caddy never reached a running state" >&2
  docker logs --tail 100 orvo-caddy || true
  exit 1
fi

docker compose ps
