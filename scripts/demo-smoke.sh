#!/usr/bin/env bash
set -euo pipefail
export PATH="${HOME}/.docker/bin:${PATH}"
API="${API_URL:-http://localhost:8080}"

echo "== ready =="
curl -sf "${API}/readyz" | tee /dev/stderr | grep -q ready

echo "== campaign =="
curl -sf "${API}/api/v1/campaign"

echo "== pay below min =="
curl -sf -X POST "${API}/api/v1/payments" \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: user_a' \
  -H "Idempotency-Key: smoke-below-$(date +%s)" \
  -d '{"amountIdr":19999}'

echo
echo "== pay 100000 =="
KEY="smoke-pay-$(date +%s)"
curl -sf -X POST "${API}/api/v1/payments" \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: user_a' \
  -H "Idempotency-Key: ${KEY}" \
  -d '{"amountIdr":100000}'

echo
echo "== idempotent replay =="
curl -sf -X POST "${API}/api/v1/payments" \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: user_a' \
  -H "Idempotency-Key: ${KEY}" \
  -d '{"amountIdr":100000}'

echo
echo "== balance =="
curl -sf -H 'X-User-ID: user_a' "${API}/api/v1/me/cashback"
echo
echo "OK"
