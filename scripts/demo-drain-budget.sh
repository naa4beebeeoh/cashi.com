#!/usr/bin/env bash
# Demo helper: set how much campaign budget is left so you can verify the Expo
# campaign strip near empty / exhausted / fully reset.
#
# Default uses a SQL fixture (fast). Pass --via-pay for the real earn path
# (best when draining only a small remaining amount).
#
# Usage:
#   ./scripts/demo-drain-budget.sh --leave 5000       # nearly empty, still active
#   ./scripts/demo-drain-budget.sh --leave 0          # exhausted
#   ./scripts/demo-drain-budget.sh --leave 10000000   # full budget again
#   ./scripts/demo-drain-budget.sh --leave 5000 --via-pay
set -euo pipefail
cd "$(dirname "$0")/../backend"

if [[ $# -eq 0 ]]; then
  echo "Usage: $0 --leave N [--via-pay]" >&2
  echo "  --leave 5000      nearly empty" >&2
  echo "  --leave 0         exhausted" >&2
  echo "  --leave 10000000  full reset" >&2
  exit 1
fi

ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --leave|-leave)
      ARGS+=(-leave "$2")
      shift 2
      ;;
    --via-pay|-via-pay)
      ARGS+=(-via-pay)
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      echo "Usage: $0 --leave N [--via-pay]" >&2
      exit 1
      ;;
  esac
done

exec go run ./cmd/demodrain "${ARGS[@]}"
