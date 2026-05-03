#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${DDI_PYTHON_CMD:-}" ]]; then
  read -r -a PYTHON_CMD <<< "$DDI_PYTHON_CMD"
elif command -v python >/dev/null 2>&1; then
  PYTHON_CMD=(python)
else
  PYTHON_CMD=(conda run -n ddi-agent python)
fi
RUNTIME_ADDR="${DDI_RUNTIME_GRPC_ADDR:-[::]:50052}"
GATEWAY_GRPC_ADDR="${DDI_GRPC_ADDR:-127.0.0.1:50052}"
HTTP_ADDR="${DDI_HTTP_ADDR:-:8080}"

docker compose up -d postgres etcd minio milvus
./scripts/init_db.sh

cleanup() {
  if [[ -n "${RUNTIME_PID:-}" ]]; then kill "$RUNTIME_PID" 2>/dev/null || true; fi
  if [[ -n "${GATEWAY_PID:-}" ]]; then kill "$GATEWAY_PID" 2>/dev/null || true; fi
}
trap cleanup EXIT INT TERM

DDI_RUNTIME_GRPC_ADDR="$RUNTIME_ADDR" "${PYTHON_CMD[@]}" python/runtime/server.py &
RUNTIME_PID=$!

DDI_GRPC_ADDR="$GATEWAY_GRPC_ADDR" DDI_HTTP_ADDR="$HTTP_ADDR" go run ./cmd/gateway &
GATEWAY_PID=$!

echo "runtime pid=$RUNTIME_PID addr=$RUNTIME_ADDR"
echo "gateway pid=$GATEWAY_PID http=$HTTP_ADDR grpc=$GATEWAY_GRPC_ADDR"
wait
