#!/usr/bin/env bash
# Activate an environment profile by copying templates into local .env files.
# Usage: ./scripts/use-env.sh demo|staging|production
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_NAME="${1:-}"
case "$ENV_NAME" in
  demo|staging|production) ;;
  *)
    echo "Usage: $0 demo|staging|production" >&2
    exit 1
    ;;
esac

SRC="env/${ENV_NAME}"
if [[ ! -f "${SRC}/backend.env.example" || ! -f "${SRC}/frontend.env.example" ]]; then
  echo "Missing templates under ${SRC}/" >&2
  exit 1
fi

cp "${SRC}/backend.env.example" backend/.env
cp "${SRC}/frontend.env.example" frontend/.env

echo "Activated ${ENV_NAME}:"
echo "  backend/.env  ← ${SRC}/backend.env.example"
echo "  frontend/.env ← ${SRC}/frontend.env.example"
if [[ "$ENV_NAME" != "demo" ]]; then
  echo "Replace CHANGE_ME / example hosts with real values before running."
fi
