#!/usr/bin/env bash
set -euo pipefail

PG_DSN="${DDI_PG_DSN:-postgres://ddi:ddi@localhost:5432/ddi?sslmode=disable}"

if command -v psql >/dev/null 2>&1; then
  psql "$PG_DSN" -f sql/audit.sql
else
  docker compose exec -T postgres psql -U ddi -d ddi < sql/audit.sql
fi
