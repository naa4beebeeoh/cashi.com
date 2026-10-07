-- Flash Cashback schema. All money is integer IDR (no floats).

CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE campaigns (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    rate_bps        INTEGER NOT NULL CHECK (rate_bps >= 0),
    min_payment_idr BIGINT NOT NULL CHECK (min_payment_idr >= 0),
    daily_cap_idr   BIGINT NOT NULL CHECK (daily_cap_idr >= 0),
    budget_total_idr BIGINT NOT NULL CHECK (budget_total_idr >= 0),
    budget_spent_idr BIGINT NOT NULL DEFAULT 0 CHECK (budget_spent_idr >= 0),
    status          TEXT NOT NULL CHECK (status IN ('active', 'exhausted')),
    timezone        TEXT NOT NULL DEFAULT 'Asia/Jakarta',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT budget_not_overspent CHECK (budget_spent_idr <= budget_total_idr)
);

CREATE TABLE wallets (
    user_id        TEXT PRIMARY KEY REFERENCES users(id),
    available_idr  BIGINT NOT NULL DEFAULT 0 CHECK (available_idr >= 0),
    redeemed_idr   BIGINT NOT NULL DEFAULT 0 CHECK (redeemed_idr >= 0),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE payments (
    id              UUID PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id),
    amount_idr      BIGINT NOT NULL CHECK (amount_idr > 0),
    cashback_idr    BIGINT NOT NULL DEFAULT 0 CHECK (cashback_idr >= 0),
    award_reason    TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, idempotency_key)
);

CREATE TABLE redemptions (
    id              UUID PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id),
    amount_idr      BIGINT NOT NULL CHECK (amount_idr > 0),
    idempotency_key TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, idempotency_key)
);

CREATE TABLE ledger_entries (
    id             UUID PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id),
    entry_type     TEXT NOT NULL CHECK (entry_type IN ('cashback_credit', 'redeem_debit')),
    amount_idr     BIGINT NOT NULL CHECK (amount_idr > 0),
    payment_id     UUID REFERENCES payments(id),
    redemption_id  UUID REFERENCES redemptions(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ledger_entries_user_created_idx ON ledger_entries (user_id, created_at DESC);

CREATE TABLE user_daily_earnings (
    user_id    TEXT NOT NULL REFERENCES users(id),
    day        DATE NOT NULL,
    earned_idr BIGINT NOT NULL DEFAULT 0 CHECK (earned_idr >= 0),
    PRIMARY KEY (user_id, day)
);

CREATE TABLE idempotency_records (
    scope           TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    response_json   JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, user_id, idempotency_key)
);

-- Seed demo users and flash campaign.
INSERT INTO users (id, display_name) VALUES
    ('user_a', 'Ayu'),
    ('user_b', 'Budi');

INSERT INTO wallets (user_id) VALUES ('user_a'), ('user_b');

INSERT INTO campaigns (
    id, name, rate_bps, min_payment_idr, daily_cap_idr,
    budget_total_idr, budget_spent_idr, status, timezone
) VALUES (
    'flash_v1',
    'Flash Cashback',
    500,
    20000,
    50000,
    10000000,
    0,
    'active',
    'Asia/Jakarta'
);
