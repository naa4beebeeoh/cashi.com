#!/usr/bin/env bash
# Reset local demo data.
#
# Usage:
#   ./scripts/demo-reset.sh           # same as: seed
#   ./scripts/demo-reset.sh seed      # wipe Compose volumes + re-seed (Ayu/Budi, full budget)
#   ./scripts/demo-reset.sh daily     # zero today's user_daily_earnings (Jakarta day); keep wallets/budget
set -euo pipefail
export PATH="${HOME}/.docker/bin:/usr/local/bin:${PATH}"
cd "$(dirname "$0")/.."

MODE="${1:-seed}"
case "$MODE" in
  seed|daily) ;;
  *)
    echo "Usage: $0 [seed|daily]" >&2
    echo "  seed   (default) docker compose down -v && up — full seed from migrations" >&2
    echo "  daily  clear Asia/Jakarta today's daily-cap counters only" >&2
    exit 1
    ;;
esac

wait_ready() {
  echo "Waiting for Postgres + Redis..."
  for _ in $(seq 1 30); do
    if docker compose exec -T postgres pg_isready -U cashi -d cashi >/dev/null 2>&1 \
      && docker compose exec -T redis redis-cli ping >/dev/null 2>&1; then
      echo "Postgres + Redis are ready."
      return 0
    fi
    sleep 1
  done
  echo "Timed out waiting for containers" >&2
  exit 1
}

if [[ "$MODE" == "seed" ]]; then
  echo "== seed reset: wipe volumes and re-apply migrations =="
  docker compose down -v
  docker compose up -d
  wait_ready
  docker compose exec -T postgres psql -U cashi -d cashi -c \
    "SELECT id, budget_spent_idr, budget_total_idr, status FROM campaigns;"
  docker compose exec -T postgres psql -U cashi -d cashi -c \
    "SELECT id, display_name FROM users ORDER BY id;"
  echo "OK — seed restored (restart API if it was running)."
  exit 0
fi

echo "== daily reset: zero today's user_daily_earnings (Asia/Jakarta) =="
if ! docker compose exec -T postgres pg_isready -U cashi -d cashi >/dev/null 2>&1; then
  echo "Postgres is not up. Run: docker compose up -d" >&2
  exit 1
fi

docker compose exec -T postgres psql -U cashi -d cashi <<'SQL'
SELECT
  COUNT(*) AS rows_today,
  COALESCE(SUM(earned_idr), 0) AS earned_sum
FROM user_daily_earnings
WHERE day = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::date;

UPDATE user_daily_earnings
SET earned_idr = 0
WHERE day = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::date;

SELECT
  COUNT(*) AS rows_today,
  COALESCE(SUM(earned_idr), 0) AS earned_sum_after
FROM user_daily_earnings
WHERE day = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Jakarta')::date;
SQL

echo "OK — daily caps cleared for Jakarta today (wallets/campaign unchanged)."
