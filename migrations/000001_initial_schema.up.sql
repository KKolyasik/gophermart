CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login      VARCHAR(256) NOT NULL UNIQUE,
    password   VARCHAR(256) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE balances (
    user_id   UUID PRIMARY KEY REFERENCES users(id),
    current   NUMERIC(15,2) NOT NULL DEFAULT 0,
    withdrawn NUMERIC(15,2) NOT NULL DEFAULT 0
);

CREATE TABLE orders (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id),
    number      VARCHAR(64) NOT NULL UNIQUE,
    status      VARCHAR(16) NOT NULL DEFAULT 'NEW',
    accrual     NUMERIC(15,2),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_user_id ON orders(user_id);

CREATE TABLE withdrawals (
    id           BIGSERIAL PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id),
    order_number VARCHAR(64) NOT NULL,
    sum          NUMERIC(15,2) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_withdrawals_user_id ON withdrawals(user_id);