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

## Frontend / Expo UI (learned from brand + Android polish)

Production-minded MVP: the main screen is a **customer spend/earn flow**; interview/ops knobs stay behind **Dev**. Match live Cashi brand (`cashi.com` / app stores), not a green admin console.

### Brand colour and layout

- **Palette**: signature orange card `#FF5C00`, white page `#FFFFFF`, black text `#171719`, muted grey for secondary copy. Avoid cream/green “fintech demo” themes unless the human asks.
- **Hero**: orange payment-card block with white lowercase **cashi** wordmark (chip + Flash Cashback + VISA-style footer). Brand must read as Cashi after removing nav chrome.
- **Android status bar**: RN `SafeAreaView` alone is not enough — pad top with `StatusBar.currentHeight` on Android so the black wordmark is not clipped.
- **Card geometry**: use `width` / `maxWidth` + `aspectRatio` (~1.586) and center with a wrapper (`alignItems: 'center'`). Do **not** pair `aspectRatio` with `maxHeight` on Android — height caps break width and look off-center.
- **CTAs**: orange pill for primary Pay; black outline for secondary Redeem.

### Customer flow vs developer tools

- **Customer surface**: Spotify Gift Card (editable load amount) → “You’ll earn ≈ …” preview → market T&C → Pay → green reward pill → wallet + redeem.
- **Dev drawer** (not the happy path): switch Ayu/Budi, API ready URL, campaign budget bar, daily usage, ledger. Label it as interview/ops helpers.
- Merchandise is a **client staging fixture** (catalogue remains out of API scope). Prefer one gift-card SKU unless the human asks for more.

### Integer IDR inputs

- Money fields: **digits only** (`replace(/\D/g, '')`), `keyboardType="number-pad"`, soft **max** (e.g. Rp2.000.000 for gift-card load) to stop fat-finger absurd amounts.
- Display with `id-ID` currency formatting; never use floats for IDR amounts in UI math (preview uses the same bps floor as domain: `floor(amount * 500 / 10000)` below min → 0).

### Customer-facing wording

- Write for cardholders, not engineers: “Earn 5% cashback when you spend from Rp20.000”, “Up to Rp50.000 cashback per day”, “Terms apply”.
- Avoid symbols like **≥** / “greater than or equal to”, API jargon, or campaign-budget internals on the main surface (those belong in Dev).
- Preview copy may say “≈”; note that the server can still clamp for daily cap / campaign budget.

### Software keyboard and focus (Android)

- Set Expo `android.softwareKeyboardLayoutMode` to `"resize"` in `app.json`.
- Wrap main content in `KeyboardAvoidingView` (iOS `padding`); use `ScrollView` with `keyboardShouldPersistTaps="handled"` and extra **bottom padding** while the keyboard is open (`Keyboard` show/hide listeners).
- **Scroll-to-end only when Redeem is focused.** Scrolling to end on every focus (including Spotify amount) jumps the viewport to Redeem and can steal focus on Pixel-class devices. Track focused field with a ref; Pay amount sits high enough that it should not force `scrollToEnd`.
- After keyboard show for Redeem, scroll again once padding updates so the field stays above the IME.

### Frontend tests

- Use **RNTL** (`@testing-library/react-native`) + `jest-expo`. Prefer assertions on customer copy and Dev-drawer separation over brittle layout snapshots.
- Mock `../api`; keep Pay/Redeem orchestration confidence on the backend.
- **Maestro** (device E2E): flows under `.maestro/` ([docs](https://docs.maestro.dev/), [repo](https://github.com/mobile-dev-inc/maestro)). Install with `curl -fsSL "https://get.maestro.mobile.dev" | bash` → `~/.maestro/bin` plus JDK 17 (`brew install openjdk@17`, set `JAVA_HOME` — formula is not on PATH by default). **Do not** `brew install maestro` (unrelated formula). Prefer stable `testID` → Maestro `id:` over brittle copy. Expo Go uses `openLink` + `APP_ID=host.exp.exponent` (not `launchApp` with our package). Standalone package/bundle: `com.cashi.flashcashback`. Run via `./scripts/maestro-test.sh` with Metro `EXPO_URL`.

## Demo / CI pointers

- Demo: see `README.md` (`./scripts/use-env.sh demo`, Compose, API, Expo web/Android).
- CI: PRs to `main` → `.github/workflows/backend-unit.yml` (unit only). Pushes to `main` → `.github/workflows/main-tests.yml` (backend unit + `RUN_INTEGRATION=1` + frontend Jest).
