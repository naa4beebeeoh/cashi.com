#!/usr/bin/env bash
# Run Maestro UI flows against Expo Go (or override APP_ID / EXPO_URL).
# Docs: https://docs.maestro.dev/  ·  https://github.com/mobile-dev-inc/maestro
set -euo pipefail
cd "$(dirname "$0")/.."

# Official CLI installs to ~/.maestro/bin (not Homebrew — brew's "maestro" is unrelated).
export PATH="${HOME}/.maestro/bin:${PATH}"

# Maestro needs a JDK 17+ on PATH / JAVA_HOME (macOS: brew openjdk@17 is not linked by default).
if [[ -z "${JAVA_HOME:-}" ]]; then
  if [[ -d /opt/homebrew/opt/openjdk@17 ]]; then
    export JAVA_HOME=/opt/homebrew/opt/openjdk@17
  elif [[ -d /usr/local/opt/openjdk@17 ]]; then
    export JAVA_HOME=/usr/local/opt/openjdk@17
  fi
fi
if [[ -n "${JAVA_HOME:-}" ]]; then
  export PATH="${JAVA_HOME}/bin:${PATH}"
fi

if ! command -v java >/dev/null 2>&1; then
  echo "Java Runtime not found. Maestro requires JDK 17+." >&2
  echo "  brew install openjdk@17" >&2
  echo '  export JAVA_HOME="$(brew --prefix openjdk@17)"' >&2
  echo '  export PATH="$JAVA_HOME/bin:$PATH"' >&2
  exit 1
fi

if ! command -v maestro >/dev/null 2>&1; then
  echo "maestro CLI not found (Mobile Dev Inc.). Install:" >&2
  echo '  curl -fsSL "https://get.maestro.mobile.dev" | bash' >&2
  echo '  export PATH="$HOME/.maestro/bin:$PATH"   # add to ~/.zshrc' >&2
  echo "Requires Java 17+. Do not use: brew install maestro" >&2
  exit 1
fi

# Sanity: real Maestro prints a version and supports "test".
if ! maestro --help 2>&1 | grep -qE 'test|cloud|studio'; then
  echo "Found a 'maestro' binary that does not look like Mobile Dev Inc. Maestro." >&2
  echo "Check: which -a maestro" >&2
  echo "Uninstall Homebrew's unrelated formula if needed: brew uninstall maestro" >&2
  exit 1
fi

APP_ID="${APP_ID:-host.exp.exponent}"
EXPO_URL="${EXPO_URL:-exp://127.0.0.1:8081}"
FLOW_PATH="${1:-.maestro/}"

# Android emulator: 127.0.0.1 is the guest, not the host. Expo Go needs adb reverse
# so exp://127.0.0.1:8081 reaches Metro (and :8080 the API) on the Mac.
if command -v adb >/dev/null 2>&1 && adb devices 2>/dev/null | grep -qE 'emulator|device$'; then
  adb reverse tcp:8081 tcp:8081 >/dev/null 2>&1 || true
  adb reverse tcp:8080 tcp:8080 >/dev/null 2>&1 || true
fi

# Pay→redeem needs headroom under the daily cap (Ayu may already be maxed from demos).
RESET_DAILY="${RESET_DAILY:-1}"
if [[ "${RESET_DAILY}" == "1" ]] && [[ -x ./scripts/demo-reset.sh ]]; then
  echo "RESET_DAILY=1 → ./scripts/demo-reset.sh daily"
  ./scripts/demo-reset.sh daily || echo "warn: daily reset failed (is Compose up?)" >&2
fi

echo "maestro test  APP_ID=${APP_ID}  EXPO_URL=${EXPO_URL}  path=${FLOW_PATH}"

# One emulator cannot run flows in parallel (shared Expo Go UI). Expand a directory
# into sequential file runs; a single file still goes straight to maestro.
run_one() {
  local flow="$1"
  echo "==> ${flow}"
  maestro test \
    -e "APP_ID=${APP_ID}" \
    -e "EXPO_URL=${EXPO_URL}" \
    "${flow}"
}

if [[ -d "${FLOW_PATH}" ]]; then
  shopt -s nullglob
  flows=()
  for f in "${FLOW_PATH}"/*.yaml "${FLOW_PATH}"/*.yml; do
    base="$(basename "${f}")"
    # Workspace config is not a flow.
    [[ "${base}" == "config.yaml" || "${base}" == "config.yml" ]] && continue
    flows+=("${f}")
  done
  if [[ ${#flows[@]} -eq 0 ]]; then
    echo "No *.yaml flows in ${FLOW_PATH}" >&2
    exit 1
  fi
  status=0
  for flow in "${flows[@]}"; do
    if ! run_one "${flow}"; then
      status=1
    fi
  done
  exit "${status}"
fi

run_one "${FLOW_PATH}"
