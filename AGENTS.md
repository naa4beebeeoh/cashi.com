# Agent guide — cashi.com (Flash Cashback)

Instructions for any AI coding agent working in this repository. Do **not** assume the human uses Cursor or any specific IDE. Prefer portable conventions (`AGENTS.md`, standard Go/Expo layouts) over vendor-only config.

## Product context

Take-home MVP: payments earn cashback under campaign rules (5%, min payment, daily cap, campaign budget), users redeem balances. Stack: Go, PostgreSQL, Redis, Expo React Native.

Brief PDF in repo root (if present): `Cashi - Sr. Software Engineer (Go_React Native) .pdf`. Scope is Flash Cashback only; auth/refunds/catalogue are out of scope unless the human asks.

## Testing principles (required)

Unit tests exist to **lock business invariants and anti-abuse edges**, not to inflate coverage metrics or mirror production line-by-line.

1. **Prefer decision-table / table-driven tests** for money and policy (`domain.ComputeAward`, `CashbackForPayment`).
2. **Every award-policy case must state inputs explicitly** — at minimum `AmountIDR`, `EarnedTodayIDR`, `BudgetSpentIDR` (no silent zero defaults that hide intent).
3. **Assert amount and reason together** when the API exposes both (wrong reason with right rupiah is still a bug).
4. **Include adversarial / gaming edges** in the same suite as happy paths (whale payments, below-min while campaign dead, exact leftovers, equal dual clamps, corrupt spent > total, zero/negative amounts). Do **not** silo them under a separate “anti-gaming” section as if they were optional.
5. **Do not write tests that only exercise branches** without a business meaning. If a case exists only because a `switch` arm needs a hit, rewrite or drop it.
6. **Smell → production fix**: if a test review finds magic strings, ambiguous policy, or reason-code inconsistency, improve production code (e.g. typed constants), then align tests — do not only paper over it in the test.
7. **Layer correctly**:
   - Pure policy → `internal/domain` unit tests
   - HTTP contracts → `httptest` + stubs (no Docker)
   - Races, idempotency under load, real Postgres/Redis → integration (`RUN_INTEGRATION=1`)
8. **Coverage is a signal, not the goal.** 100% on pure money helpers is expected; low % on orchestration that needs infra is acceptable when integration covers it.

## Code style

- Match existing package layout and naming (`domain_test.go` next to `domain.go`).
- Integer IDR only for money; no floats.
- Keep diffs focused; no drive-by refactors or unsolicited markdown docs.
- Do not commit secrets. Env templates live under `env/<demo|staging|production>/`; local `.env` is gitignored.

## Demo / CI pointers

- Demo: see `README.md` (`./scripts/use-env.sh demo`, Compose, API, Expo web/Android).
- CI: `.github/workflows/backend-unit.yml` runs backend unit tests on pushes/PRs to `main` and uploads coverage/JUnit artifacts.
