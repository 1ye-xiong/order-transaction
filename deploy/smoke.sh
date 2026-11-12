#!/usr/bin/env bash
# A full isolated HTTP rehearsal uses real services and checks shipment, refund,
# late payment, concurrency and persistence. Production smoke remains non-destructive.
set -euo pipefail
BASE=${BASE:-http://127.0.0.1:8080/api/v1}
curl -fsS "${BASE%/api/v1}/healthz" >/dev/null
response=$(curl -fsS -X POST "$BASE/users/login" -H 'Content-Type: application/json' -d '{"account":"buyer1","password":"123456"}')
TOKEN=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])' <<< "$response")
curl -fsS "$BASE/products" -H "Authorization: Bearer $TOKEN" >/dev/null
curl -fsS "$BASE/orders/mine" -H "Authorization: Bearer $TOKEN" >/dev/null
echo "Smoke PASS: health/login/products/owned orders (no stock consumed)"
