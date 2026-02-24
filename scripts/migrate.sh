#!/bin/sh
set -eu

require_var() {
	name="$1"
	eval "value=\${$name:-}"
	if [ -z "$value" ]; then
		echo "missing required env var: $name" >&2
		exit 1
	fi
}

POSTGRES_DSN="${POSTGRES_URL:-${DATABASE_URL:-}}"
if [ -z "$POSTGRES_DSN" ]; then
	echo "missing required env var: POSTGRES_URL (or DATABASE_URL)" >&2
	exit 1
fi

echo "running postgres migrations..."
goose -dir /app/migrations/postgres postgres "$POSTGRES_DSN" up

echo "migrations complete"
