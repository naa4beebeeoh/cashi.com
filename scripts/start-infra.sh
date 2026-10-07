#!/usr/bin/env bash
# Run this in your own Terminal (Docker Desktop must be running).
set -euo pipefail
export PATH="${HOME}/.docker/bin:/usr/local/bin:${PATH}"
cd "$(dirname "$0")/.."
docker compose up -d
docker compose ps
echo "Waiting for health..."
for i in $(seq 1 30); do
  if docker compose exec -T postgres pg_isready -U cashi -d cashi >/dev/null 2>&1 \
    && docker compose exec -T redis redis-cli ping >/dev/null 2>&1; then
    echo "Postgres + Redis are ready."
    exit 0
  fi
  sleep 1
done
echo "Timed out waiting for containers" >&2
exit 1
