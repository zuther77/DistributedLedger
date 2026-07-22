-- Users: people with a cash balance 
CREATE TABLE if NOT EXISTS users (
    id              UUID PRIMARY KEY,
    balance         NUMERIC(18, 2) NOT NULL DEFAULT 0.00,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
); 

-- Orders: a widh to buy or sell 
CREATE TABLE IF NOT EXISTS orders (
    id              UUID PRIMARY KEY,
    user_id UUID    NOT NULL REFERENCES users(id),
    ticker          TEXT NOT NULL, 
    side            TEXT NOT NULL CHECK (side IN ('BUY', 'SELL')),
    quantity        NUMERIC(18, 2) NOT NULL,
    price           NUMERIC(18, 2) NOT NULL,
    status          TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Trades: proof that a buy met a sell 
CREATE TABLE IF NOT EXISTS trades (
    id                  UUID PRIMARY KEY,
    buy_order_id        UUID NOT NULL REFERENCES orders(id),
    sell_order_id       UUID NOT NULL REFERENCES orders(id),
    ticker              TEXT NOT NULL, 
    execution_price     NUMERIC(18, 2) NOT NULL,
    quantity            NUMERIC(18, 2) NOT NULL,
    executed_at         TIMESTAMPTZ NOT NULL DEFAULT now()
); 


-- find orders by status faster)
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);

-- find trades by buy order faster
CREATE INDEX IF NOT EXISTS idx_trades_buy_order_id ON trades(buy_order_id);

-- find trades by sell order faster
CREATE INDEX IF NOT EXISTS idx_trades_sell_order_id ON trades(sell_order_id);

-- Demo users (smoke.py / curl labs). Only runs on first Postgres boot (empty volume).
INSERT INTO users (id, balance) VALUES
  ('11111111-1111-1111-1111-111111111111', 10000.00), -- buyer
  ('22222222-2222-2222-2222-222222222222', 10000.00)  -- seller
ON CONFLICT (id) DO NOTHING;
