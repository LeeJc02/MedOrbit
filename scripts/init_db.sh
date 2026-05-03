#!/usr/bin/env bash
set -euo pipefail

PG_DSN="${DDI_PG_DSN:-postgres://ddi:ddi@127.0.0.1:5432/ddi?sslmode=disable}"

wait_for_host_postgres() {
  for _ in $(seq 1 60); do
    if PGCONNECT_TIMEOUT=2 psql "$PG_DSN" -c "select 1" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  echo "postgres did not become ready on ${PG_DSN}" >&2
  return 1
}

wait_for_container_postgres() {
  for _ in $(seq 1 60); do
    if docker compose exec -T postgres pg_isready -U ddi -d ddi >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  echo "postgres container did not become ready in time" >&2
  return 1
}

if command -v psql >/dev/null 2>&1; then
  wait_for_host_postgres
  PGCONNECT_TIMEOUT=5 psql "$PG_DSN" -f sql/audit.sql
else
  wait_for_container_postgres
  docker compose exec -T postgres psql -U ddi -d ddi < sql/audit.sql
fi
