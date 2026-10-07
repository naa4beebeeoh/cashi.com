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

```sh
export PATH="$HOME/.docker/bin:$PATH"   # if needed on macOS
docker ps                               # must succeed before continuing
```

## Run the demo

Open three terminals from the repo root.

```sh
# 1) Postgres + Redis (applies backend/migrations on first boot)
docker compose up -d
# or: ./scripts/start-infra.sh
```

```sh
# 2) API → http://localhost:8080
cd backend
cp -n .env.example .env   # optional; defaults match Compose
go run ./cmd/api
```

```sh
# 3) Web client → http://localhost:8081
cd frontend
cp -n .env.example .env
npm install
npm run web
```

Then open **http://localhost:8081** in a browser. Pick a seed user in the UI and try a payment / redeem.

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

```sh
cd backend && go test ./internal/domain/...

# integration (Compose must be up)
cd backend && RUN_INTEGRATION=1 go test ./internal/cashback/... -count=1

cd frontend && npm test
```

## Reset demo data

```sh
docker compose down -v
docker compose up -d
```

## Layout

```
backend/cmd/api          # process entry
backend/internal/...     # config, domain, cashback service, httpapi, postgres, redis
backend/migrations       # SQL applied by Compose on first start
frontend/                # Expo app (Flash Cashback UI)
scripts/                 # infra start + API smoke helpers
```
