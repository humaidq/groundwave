-- +goose Up
-- Add ledger tables for budgets, accounts, transactions, and reconciliations

-- +goose StatementBegin
DO $$ BEGIN
    CREATE TYPE ledger_account_type AS ENUM ('regular', 'debt', 'tracking');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$ BEGIN
    CREATE TYPE ledger_transaction_status AS ENUM ('pending', 'cleared', 'refunded', 'rejected');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;
-- +goose StatementEnd

--------------------------------------------------------------------------------
-- BUDGETS (monthly)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ledger_budgets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_name TEXT NOT NULL
                  CONSTRAINT ledger_budget_name_not_empty CHECK (length(trim(category_name)) > 0),
    amount        NUMERIC(12,2) NOT NULL,
    currency      TEXT NOT NULL DEFAULT 'AED',
    period_start  DATE NOT NULL,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT ledger_budget_unique_period UNIQUE (category_name, period_start)
);

CREATE INDEX IF NOT EXISTS idx_ledger_budgets_period_start ON ledger_budgets(period_start);
CREATE INDEX IF NOT EXISTS idx_ledger_budgets_category ON ledger_budgets(category_name);

DROP TRIGGER IF EXISTS ledger_budgets_updated_at ON ledger_budgets;
CREATE TRIGGER ledger_budgets_updated_at
    BEFORE UPDATE ON ledger_budgets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

--------------------------------------------------------------------------------
-- ACCOUNTS
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ledger_accounts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL
                    CONSTRAINT ledger_account_name_not_empty CHECK (length(trim(name)) > 0),
    account_type    ledger_account_type NOT NULL,
    opening_balance NUMERIC(12,2) NOT NULL DEFAULT 0,
    iban            TEXT,
    bank_name       TEXT,
    account_number  TEXT,
    description     TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_accounts_type ON ledger_accounts(account_type);
CREATE INDEX IF NOT EXISTS idx_ledger_accounts_name ON ledger_accounts(name);

DROP TRIGGER IF EXISTS ledger_accounts_updated_at ON ledger_accounts;
CREATE TRIGGER ledger_accounts_updated_at
    BEFORE UPDATE ON ledger_accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

--------------------------------------------------------------------------------
-- TRANSACTIONS
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ledger_transactions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id   UUID NOT NULL REFERENCES ledger_accounts(id) ON DELETE CASCADE,
    budget_id    UUID REFERENCES ledger_budgets(id) ON DELETE SET NULL,
    amount       NUMERIC(12,2) NOT NULL,
    merchant     TEXT NOT NULL
                 CONSTRAINT ledger_transaction_merchant_not_empty CHECK (length(trim(merchant)) > 0),
    status       ledger_transaction_status NOT NULL DEFAULT 'cleared',
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    note         TEXT,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_transactions_account ON ledger_transactions(account_id);
CREATE INDEX IF NOT EXISTS idx_ledger_transactions_budget ON ledger_transactions(budget_id);
CREATE INDEX IF NOT EXISTS idx_ledger_transactions_occurred ON ledger_transactions(occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_ledger_transactions_status ON ledger_transactions(status);

DROP TRIGGER IF EXISTS ledger_transactions_updated_at ON ledger_transactions;
CREATE TRIGGER ledger_transactions_updated_at
    BEFORE UPDATE ON ledger_transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

--------------------------------------------------------------------------------
-- RECONCILIATIONS
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ledger_reconciliations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id    UUID NOT NULL REFERENCES ledger_accounts(id) ON DELETE CASCADE,
    balance       NUMERIC(12,2) NOT NULL,
    reconciled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    note          TEXT,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_reconciliations_account ON ledger_reconciliations(account_id);
CREATE INDEX IF NOT EXISTS idx_ledger_reconciliations_time ON ledger_reconciliations(reconciled_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ledger_reconciliations;
DROP TABLE IF EXISTS ledger_transactions;
DROP TABLE IF EXISTS ledger_accounts;
DROP TABLE IF EXISTS ledger_budgets;
DROP TYPE IF EXISTS ledger_transaction_status;
DROP TYPE IF EXISTS ledger_account_type;
