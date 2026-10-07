#!/usr/bin/env bash
# Production-support style daily audit for Flash Cashback.
# Reads DATABASE_URL from the environment or backend/.env (via config.Load).
#
# Usage:
#   ./scripts/ops-daily-audit.sh
#   ./scripts/ops-daily-audit.sh --day 2026-10-07
#   ./scripts/ops-daily-audit.sh --out reports/audit
#
# Exit codes:
#   0  invariants OK
#   1  invariant failure (budget / daily cap)
#   2  tool / connectivity error
set -euo pipefail
cd "$(dirname "$0")/../backend"

# Default under backend/reports (gitignored) so audit logs are not committed.
OUT_DEFAULT="reports/audit"
ARGS=("$@")
if [[ $# -eq 0 ]]; then
  ARGS=(--out "${OUT_DEFAULT}")
fi

mkdir -p "${OUT_DEFAULT}"
exec go run ./cmd/opsaudit "${ARGS[@]}"
