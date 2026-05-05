#!/usr/bin/env bash
set -euo pipefail

if [ -z "${ORVO_BOOTSTRAP_NAME:-}" ] || [ -z "${ORVO_BOOTSTRAP_EMAIL:-}" ] || [ -z "${ORVO_BOOTSTRAP_PASSWORD:-}" ]; then
  echo "bootstrap credentials are not configured, skipping org/api-key bootstrap" >&2
  exit 0
fi

base_url="${BASE_URL:-http://127.0.0.1:${ORVO_HTTP_PORT:-80}}"
shared_env_file="${SHARED_TELEMETRY_ENV_FILE:-/opt/orvo/shared/orvo-telemetry.env}"
runtime_env_file="${RUNTIME_COLLECTOR_ENV_FILE:-runtime/collector.env}"

if [ -f "$shared_env_file" ] && grep -q '^ORVO_OTEL_API_KEY=' "$shared_env_file"; then
  mkdir -p "$(dirname "$runtime_env_file")"
  cp "$shared_env_file" "$runtime_env_file"
  exit 0
fi

auth_headers="$(mktemp)"
session_json="$(mktemp)"
create_key_json="$(mktemp)"
trap 'rm -f "$auth_headers" "$session_json" "$create_key_json"' EXIT
session_cookie_name="${SESSION_COOKIE_NAME:-orvo_sess}"

extract_session_token() {
  python3 - "$1" "$session_cookie_name" <<'PY'
import sys

headers_path = sys.argv[1]
cookie_name = sys.argv[2]

with open(headers_path, "r", encoding="utf-8") as handle:
    for line in handle:
        if not line.lower().startswith("set-cookie:"):
            continue
        value = line.split(":", 1)[1].strip()
        for chunk in value.split(";"):
            chunk = chunk.strip()
            prefix = cookie_name + "="
            if chunk.startswith(prefix):
                print(chunk[len(prefix):])
                raise SystemExit(0)

print("")
PY
}

register_payload="$(python3 - <<'PY'
import json
import os

print(json.dumps({
    "name": os.environ["ORVO_BOOTSTRAP_NAME"],
    "email": os.environ["ORVO_BOOTSTRAP_EMAIL"],
    "password": os.environ["ORVO_BOOTSTRAP_PASSWORD"],
}))
PY
)"

login_payload="$(python3 - <<'PY'
import json
import os

print(json.dumps({
    "email": os.environ["ORVO_BOOTSTRAP_EMAIL"],
    "password": os.environ["ORVO_BOOTSTRAP_PASSWORD"],
}))
PY
)"

register_status="$(curl -sS -D "$auth_headers" -o /dev/null -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d "$register_payload" \
  "$base_url/api/v1/auth/register")"
session_token="$(extract_session_token "$auth_headers")"

if [ "${register_status}" -lt 200 ] || [ "${register_status}" -ge 300 ]; then
  login_status="$(curl -sS -D "$auth_headers" -o /dev/null -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -d "$login_payload" \
    "$base_url/api/v1/auth/login")"
  session_token="$(extract_session_token "$auth_headers")"

  if [ "${login_status}" -lt 200 ] || [ "${login_status}" -ge 300 ]; then
    echo "failed to register or login bootstrap user" >&2
    exit 1
  fi
fi

if [ -z "$session_token" ]; then
  echo "failed to capture bootstrap session cookie" >&2
  exit 1
fi

curl -sS -H "Cookie: ${session_cookie_name}=${session_token}" "$base_url/api/v1/auth/session" >"$session_json"
organization_id="$(python3 - "$session_json" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

active_org = payload.get("active_organization") or {}
print(active_org.get("id", ""))
PY
)"

if [ -z "$organization_id" ]; then
  echo "bootstrap user does not have an active organization" >&2
  exit 1
fi

create_key_payload='{"name":"hosted-telemetry"}'
curl -sS -H "Cookie: ${session_cookie_name}=${session_token}" \
  -H 'Content-Type: application/json' \
  -d "$create_key_payload" \
  "$base_url/api/v1/organizations/${organization_id}/api-keys" >"$create_key_json"

otel_api_key="$(python3 - "$create_key_json" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

print(payload.get("key", ""))
PY
)"

if [ -z "$otel_api_key" ]; then
  echo "failed to create the collector/chat telemetry API key" >&2
  exit 1
fi

mkdir -p "$(dirname "$shared_env_file")" "$(dirname "$runtime_env_file")"
cat >"$shared_env_file" <<EOF
ORVO_ORGANIZATION_ID=${organization_id}
ORVO_OTEL_API_KEY=${otel_api_key}
ORVO_OTLP_GRPC_ENDPOINT=orvo-app:4317
ORVO_OTLP_HTTP_ENDPOINT=http://orvo-app:4318
EOF
cp "$shared_env_file" "$runtime_env_file"
