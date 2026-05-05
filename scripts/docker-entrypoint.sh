#!/bin/sh
set -eu

if [ "${RUN_DB_MIGRATIONS:-true}" = "true" ]; then
  attempts="${DB_MIGRATION_ATTEMPTS:-30}"
  delay="${DB_MIGRATION_DELAY_SECONDS:-2}"
  current_attempt=1

  while [ "$current_attempt" -le "$attempts" ]; do
    if /app/migrate.sh; then
      break
    fi

    if [ "$current_attempt" -eq "$attempts" ]; then
      echo "database migrations failed after ${attempts} attempts" >&2
      exit 1
    fi

    echo "database is not ready yet, retrying migrations (${current_attempt}/${attempts})..." >&2
    sleep "$delay"
    current_attempt=$((current_attempt + 1))
  done
fi

exec /app/orvo

