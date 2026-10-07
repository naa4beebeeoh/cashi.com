# cashi.com — Flash Cashback

Take-home MVP: users pay amounts in IDR, earn 5% cashback under campaign rules, and redeem balances.

**Stack:** Go · PostgreSQL · Redis · Expo React Native

## Rules

- 5% cashback on payments ≥ **Rp20.000**
- Per-user daily earn cap **Rp50.000** (`Asia/Jakarta` day)
- Campaign budget **Rp10.000.000**; when spent, campaign is exhausted
- Users can redeem available cashback (ledger debit; no bank rail)

## Prerequisites

- Go 1.22+
- Node 20+ / npm
- Docker Desktop running (Compose)
- For Android: Android Studio emulator (or a device) with Expo Go

```sh
export PATH="$HOME/.docker/bin:$PATH"   # if needed on macOS
docker ps                               # must succeed before continuing
```

## Run the demo

Open three terminals from the repo root (infra + API + client).

```sh
# 1) Postgres + Redis (applies backend/migrations on first boot)
docker compose up -d
# or: ./scripts/start-infra.sh
```

```sh
# 2) Pick an env profile (writes backend/.env + frontend/.env)
./scripts/use-env.sh demo
# other profiles: staging | production  (edit placeholders first)
```

```sh
# 3) API → http://localhost:8080
cd backend
go run ./cmd/api
```

```sh
# 4) Client (pick web and/or Android)
cd frontend
npm install
```

### Web

```sh
cd frontend
npm run web
```

Open **http://localhost:8081**. The client calls the API at `http://localhost:8080`.

### Android (emulator)

1. Start your emulator first (e.g. Pixel in Android Studio) and confirm `adb devices` shows it as `device`.
2. From `frontend`, if Metro is **not** already running:

```sh
cd frontend
npm run android
```

If you already started web (`npm run web` on port 8081), **do not** start a second Expo process. In that Metro terminal press `a`, or run `npx expo start --android` only when 8081 is free. A second `npm run android` will ask to use another port — choose the existing Metro instead.

Expo installs/opens **Expo Go** and loads the app. On the Android emulator, `localhost` is the emulator itself, so the app rewrites the API host to `http://10.0.2.2:8080` (your machine). You do not need to change `.env` for this.

### Web and Android together

One Metro server can serve both. From `frontend` (with emulator already running):

```sh
npx expo start
```

Then press `w` for web and/or `a` for Android.

Pick a seed user in the UI and try a payment / redeem.

Health check (API terminal must be running):

```sh
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
```

Seed users (send as `X-User-ID`): `user_a` (Ayu), `user_b` (Budi).

Optional API smoke (after API is up):

```sh
./scripts/demo-smoke.sh
```

### Example payment

```sh
curl -s -X POST localhost:8080/api/v1/payments \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: user_a' \
  -H 'Idempotency-Key: demo-1' \
  -d '{"amountIdr":100000}'
```

## API

| Method | Path | Notes |
|--------|------|-------|
| GET | `/healthz` | liveness |
| GET | `/readyz` | Postgres + Redis |
| GET | `/api/v1/campaign` | budget + status |
| GET | `/api/v1/me/cashback` | requires `X-User-ID` |
| GET | `/api/v1/me/ledger` | requires `X-User-ID` |
| POST | `/api/v1/payments` | body `{ "amountIdr": N }`, requires `X-User-ID` + `Idempotency-Key` |
| POST | `/api/v1/redeem` | same headers/body shape |

Money is always integer IDR. Auth is out of scope; identity is the `X-User-ID` header.

## Tests

Unit tests (no Docker) — domain award math, service validation, HTTP contracts, config fail-closed:

```sh
cd backend && go test ./internal/domain/... ./internal/config/... ./internal/httpapi/... ./internal/cashback/... -count=1
```

Local coverage HTML (Go native — there is no built-in HTML *test* report, only coverage):

```sh
cd backend
mkdir -p reports
go test ./internal/domain/... ./internal/config/... ./internal/httpapi/... ./internal/cashback/... \
  -count=1 -coverprofile=reports/coverage.out
go tool cover -html=reports/coverage.out -o reports/coverage.html
open reports/coverage.html   # macOS
```

CI: pushes and PRs to `main` run [Backend unit tests](.github/workflows/backend-unit.yml), upload `coverage.html` + JUnit as artifacts, and write results to the Actions job summary.

Integration tests (Compose must be up) — idempotency, daily cap, concurrent budget:

```sh
cd backend && RUN_INTEGRATION=1 go test ./internal/cashback/... -count=1
```

```sh
cd frontend && npm test
```

## Environments

Profiles live under `env/<name>/` as committed **examples** (no secrets). Activate one locally:

```sh
./scripts/use-env.sh demo         # local Compose defaults
./scripts/use-env.sh staging      # edit CHANGE_ME / hosts first
./scripts/use-env.sh production   # edit CHANGE_ME / hosts first
```

| Profile | `APP_ENV` | Backend defaults | Typical use |
|---------|-----------|------------------|-------------|
| `demo` | `demo` | localhost Compose URLs allowed | Interview / local demo |
| `staging` | `staging` | **required** `DATABASE_URL` + `REDIS_URL` (fail closed) | Pre-prod |
| `production` | `production` | same fail-closed rules | Prod |

Frontend knobs: `EXPO_PUBLIC_APP_ENV`, `EXPO_PUBLIC_API_URL` (baked in at Metro/Expo build time).

Deploy pipeline shape: inject the matching profile’s values as process env (or copy into `backend/.env` / `frontend/.env` in CI). Do not commit filled staging/production files — `.env` is gitignored.

## Reset demo data

```sh
docker compose down -v
docker compose up -d
```

## Ops: daily audit log

Production-support snapshot of campaign budget + Jakarta-day earn/redeem activity (invariant checks included):

```sh
# human + JSON under backend/reports/audit/YYYY-MM-DD.{txt,json}
./scripts/ops-daily-audit.sh

# stdout only, or a specific day
cd backend && go run ./cmd/opsaudit
cd backend && go run ./cmd/opsaudit -day 2026-10-07 -out reports/audit
```

Uses `DATABASE_URL` / `APP_ENV` like the API. Exit `1` if budget or daily-cap integrity fails (suitable for cron alerting).

## Layout

```
backend/cmd/api          # process entry
backend/cmd/opsaudit     # daily campaign / earn audit for ops
backend/internal/...     # config, domain, cashback service, httpapi, postgres, redis
backend/migrations       # SQL applied by Compose on first start
frontend/                # Expo app (Flash Cashback UI)
env/demo|staging|production/  # env profile templates
scripts/                 # infra start, use-env, API smoke, ops audit
```
