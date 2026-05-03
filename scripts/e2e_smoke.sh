#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${DDI_PYTHON_CMD:-}" ]]; then
  read -r -a PYTHON_CMD <<< "$DDI_PYTHON_CMD"
elif command -v python >/dev/null 2>&1; then
  PYTHON_CMD=(python)
else
  PYTHON_CMD=(conda run --no-capture-output -n ddi-agent python)
fi
RUNTIME_ADDR="${DDI_RUNTIME_GRPC_ADDR:-127.0.0.1:50052}"
GATEWAY_GRPC_ADDR="${DDI_GRPC_ADDR:-127.0.0.1:50052}"
HTTP_ADDR="${DDI_HTTP_ADDR:-127.0.0.1:18080}"
BASE_URL="http://${HTTP_ADDR}"
SESSION_ID="smoke-$(date +%s)"
JWT_SECRET="${DDI_JWT_SECRET:-dev-secret}"

make_jwt() {
  local tenant_id="$1"
  DDI_JWT_SECRET="$JWT_SECRET" DDI_JWT_TENANT_ID="$tenant_id" "${PYTHON_CMD[@]}" - <<'PY'
import base64
import hashlib
import hmac
import json
import os
import time

secret = os.environ["DDI_JWT_SECRET"].encode()
tenant_id = os.environ["DDI_JWT_TENANT_ID"]

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()

header = {"alg": "HS256", "typ": "JWT"}
payload = {
    "sub": "u1",
    "tenant_id": tenant_id,
    "roles": ["doctor"],
    "exp": int(time.time()) + 3600,
}
signing_input = f"{b64url(json.dumps(header, separators=(',', ':')).encode())}.{b64url(json.dumps(payload, separators=(',', ':')).encode())}"
signature = hmac.new(secret, signing_input.encode(), hashlib.sha256).digest()
print(f"{signing_input}.{b64url(signature)}")
PY
}

SMOKE_TOKEN="$(make_jwt tenant-smoke)"
CROSS_TENANT_TOKEN="$(make_jwt tenant-other)"

cleanup() {
  if [[ -n "${RUNTIME_PID:-}" ]]; then kill "$RUNTIME_PID" 2>/dev/null || true; fi
  if [[ -n "${GATEWAY_PID:-}" ]]; then kill "$GATEWAY_PID" 2>/dev/null || true; fi
}
trap cleanup EXIT INT TERM

docker compose up -d postgres etcd minio milvus
./scripts/init_db.sh

DDI_RUNTIME_GRPC_ADDR="$RUNTIME_ADDR" "${PYTHON_CMD[@]}" python/runtime/server.py &
RUNTIME_PID=$!

for _ in $(seq 1 30); do
  if nc -z 127.0.0.1 "${RUNTIME_ADDR##*:}" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

DDI_GRPC_ADDR="$GATEWAY_GRPC_ADDR" DDI_HTTP_ADDR="$HTTP_ADDR" DDI_JWT_SECRET="$JWT_SECRET" go run ./cmd/gateway &
GATEWAY_PID=$!

for _ in $(seq 1 50); do
  if curl -fsS "$BASE_URL/" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

curl -fsS "$BASE_URL/" >/dev/null

RUN_BODY=$(cat <<JSON
{"session_id":"$SESSION_ID","user_id":"u1","locale":"zh-CN","input_text":"aspirin and warfarin","metadata":{"smoke":"true"}}
JSON
)

RUN_RESPONSE=$(curl -fsS \
  -H "Authorization: Bearer $SMOKE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$RUN_BODY" \
  "$BASE_URL/v1/session/run")

"${PYTHON_CMD[@]}" - "$RUN_RESPONSE" <<'PY'
import json
import sys

data = json.loads(sys.argv[1])
for key in ("claims", "risk_level", "draft", "followups"):
    if key not in data:
        raise SystemExit(f"missing {key} in run response")
PY

REPLAY_RESPONSE=$(curl -fsS \
  -H "Authorization: Bearer $SMOKE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"session_id\":\"$SESSION_ID\"}" \
  "$BASE_URL/v1/session/replay")

"${PYTHON_CMD[@]}" - "$REPLAY_RESPONSE" <<'PY'
import json
import sys

data = json.loads(sys.argv[1])
events = data.get("events") or []
required = ("gateway.request:", "gateway.response:", "runtime.request:", "runtime.response:")
missing = [prefix for prefix in required if not any(event.startswith(prefix) for event in events)]
if missing:
    raise SystemExit(f"missing replay events: {missing}")
PY

CROSS_REPLAY_STATUS=$(curl -sS -o /tmp/ddi_cross_replay_response.json -w "%{http_code}" \
  -H "Authorization: Bearer $CROSS_TENANT_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"session_id\":\"$SESSION_ID\"}" \
  "$BASE_URL/v1/session/replay")

if [[ "$CROSS_REPLAY_STATUS" != "403" ]]; then
  cat /tmp/ddi_cross_replay_response.json >&2 || true
  echo "expected cross-tenant replay to return 403, got $CROSS_REPLAY_STATUS" >&2
  exit 1
fi

echo "e2e smoke passed for session $SESSION_ID"
