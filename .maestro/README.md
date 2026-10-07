# Maestro UI tests (Flash Cashback)

Black-box UI automation via [Maestro](https://github.com/mobile-dev-inc/maestro) / [docs](https://docs.maestro.dev/). Flows live here; no npm Detox/Appium dependency inside the Expo app.

## Install CLI

Use the **official installer only** ([mobile-dev-inc/maestro](https://github.com/mobile-dev-inc/maestro)). Requires **JDK 17+**.

```sh
# 1) JDK (Homebrew formula — not linked into /usr/bin by default)
brew install openjdk@17
export JAVA_HOME="$(brew --prefix openjdk@17)"
export PATH="$JAVA_HOME/bin:$PATH"

# 2) Maestro CLI
curl -fsSL "https://get.maestro.mobile.dev" | bash
export PATH="$HOME/.maestro/bin:$PATH"

# persist in ~/.zshrc:
#   export JAVA_HOME="$(brew --prefix openjdk@17)"
#   export PATH="$JAVA_HOME/bin:$HOME/.maestro/bin:$PATH"

java -version
maestro --version   # expect 2.x from mobile.dev
```

Do **not** use Homebrew `brew install maestro` — that formula is a different project. `./scripts/maestro-test.sh` auto-sets `JAVA_HOME` from `openjdk@17` when present.

Android: emulator or device with **Expo Go**. iOS Simulator needs Maestro + Facebook IDB (see Maestro docs).

## Run locally (Expo Go)

1. Infra + API: `docker compose up -d` and `cd backend && go run ./cmd/api`
2. Metro + Android: `cd frontend && npm run android` (keeps Expo Go + Metro linked)
3. Emulator: `./scripts/maestro-test.sh` runs `adb reverse` for `:8081`/`:8080` so `exp://127.0.0.1:8081` reaches the host. Physical device: use the LAN `exp://…` from Metro instead.

```sh
# from repo root — override URL / app id as needed
./scripts/maestro-test.sh
# or:
maestro test -e APP_ID=host.exp.exponent -e EXPO_URL='exp://192.168.x.x:8081' .maestro/
```

| Env | Default | Notes |
|-----|---------|--------|
| `APP_ID` | `host.exp.exponent` | Expo Go Android. iOS Expo Go: `host.exp.Exponent`. Standalone: `com.cashi.flashcashback` |
| `EXPO_URL` | `exp://127.0.0.1:8081` | Emulator needs `adb reverse` (script does this). Device: LAN IP. |
| `RESET_DAILY` | `1` | `demo-reset.sh daily` before flows so Pay→redeem isn’t blocked by a spent daily cap. Set `0` to skip. |

`./scripts/maestro-test.sh` runs flows **sequentially** (one emulator / Expo Go session). Text selectors are regex — avoid bare `/` and match `id-ID` money with patterns like `.*5\\.000 cashback reward.*` (NBSP after `Rp`).

**Expo Go** cannot use `launchApp` with our package name — flows use `openLink` (see [React Native](https://docs.maestro.dev/get-started/supported-platform/react-native)). For EAS/standalone APKs, switch to `launchApp` + `com.cashi.flashcashback`.

## Flows

| File | What it covers |
|------|----------------|
| `smoke_spend_earn.yaml` | Customer copy + Pay control visible; Dev drawer open/close |
| `pay_then_redeem.yaml` | Tap Pay → reward → Redeem (needs API) |
| `subflows/open_app.yaml` | Shared `openLink` + wait for home headline |

Selectors prefer `testID` (`pay-gift-card`, `redeem-payout`, …) mapped to Maestro `id:`.
