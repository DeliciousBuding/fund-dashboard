-- CI / local smoke seed for fund-dashboard Go backend.
-- Minimal production-shaped schema + two holdings so read APIs and admin checks work.
-- NOTE: real self-hosted first installs use internal/repository/db/schema_sqlite.go
-- (EnsureSQLiteSchema) at boot; this file only seeds CI/local fixtures.

PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS fund_details (
  fund_code TEXT PRIMARY KEY,
  fund_name TEXT,
  fund_type TEXT,
  security_type TEXT DEFAULT 'fund',
  market TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS portfolio_snapshot (
  fund_code TEXT PRIMARY KEY,
  fund_name TEXT,
  held_shares REAL,
  total_cost REAL,
  latest_nav REAL,
  current_value REAL,
  unrealized_pnl REAL,
  pnl_pct REAL,
  security_type TEXT DEFAULT 'fund',
  portfolio_id INTEGER DEFAULT 1
);

CREATE TABLE IF NOT EXISTS nav_history (
  fund_code TEXT,
  date TEXT,
  unit_nav REAL,
  daily_change_pct REAL DEFAULT 0,
  security_type TEXT DEFAULT 'fund',
  PRIMARY KEY (fund_code, date)
);

CREATE TABLE IF NOT EXISTS transactions (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id TEXT,
  trade_time TEXT,
  confirm_date TEXT,
  trade_type TEXT,
  direction TEXT,
  fund_code TEXT,
  fund_name TEXT,
  confirm_amount REAL,
  confirm_share REAL,
  fee REAL,
  signed_cash_flow REAL,
  signed_share_change REAL,
  settlement_days INTEGER
);

CREATE TABLE IF NOT EXISTS fund_holdings (
  fund_code TEXT,
  stock_code TEXT,
  stock_name TEXT,
  weight_pct REAL,
  shares REAL,
  market_value REAL,
  report_date TEXT,
  PRIMARY KEY (fund_code, stock_code, report_date)
);

CREATE TABLE IF NOT EXISTS source_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  url TEXT,
  source TEXT NOT NULL DEFAULT 'websearch',
  snippet TEXT,
  query TEXT,
  related_security_code TEXT,
  related_security_name TEXT,
  is_read INTEGER DEFAULT 0,
  is_useful INTEGER DEFAULT 0,
  fetched_at TEXT NOT NULL DEFAULT (datetime('now')),
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS indices (
  code TEXT PRIMARY KEY,
  name TEXT,
  market TEXT,
  price REAL,
  change_pct REAL,
  change_amt REAL,
  updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS portfolio_definitions (
  id INTEGER PRIMARY KEY,
  name TEXT,
  description TEXT
);

INSERT OR IGNORE INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
  ('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund', 'CN'),
  ('AAPL', 'Apple Inc.', '科技股', 'stock', 'US');

INSERT OR IGNORE INTO portfolio_snapshot
  (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
VALUES
  ('019173', '纳斯达克100指数(QDII)C', 100, -120, 1.5, 150, 30, 25, 'fund', 1),
  ('AAPL', 'Apple Inc.', 2, -300, 190, 380, 80, 26.67, 'stock', 1);

INSERT OR IGNORE INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES
  ('019173', '2026-06-18', 1.5, -4.2, 'fund'),
  ('AAPL', '2026-06-18', 190, 6.5, 'stock');

INSERT OR IGNORE INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date) VALUES
  ('019173', 'NVDA', 'NVIDIA', 8.5, 100, 12000, '2026-03-31');

INSERT OR IGNORE INTO transactions
  (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
VALUES
  ('TX001', '2026-06-01T09:00:00Z', '2026-06-02', '用户买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 120, 100, 0.1, -120.1, 100, 1),
  ('TX002', '2026-06-01T09:00:00Z', '2026-06-02', '用户买入', 'buy', 'AAPL', 'Apple Inc.', 300, 2, 0.1, -300.1, 2, 1);

INSERT OR IGNORE INTO indices (code, name, market, price, change_pct, change_amt, updated_at) VALUES
  ('^GSPC', '标普500', 'US', 5600.5, 0.42, 23.5, '2026-06-18 20:00:00'),
  ('^NDX', '纳斯达克100', 'US', 19888.2, 1.25, 245.8, '2026-06-18 20:00:00');

INSERT OR IGNORE INTO portfolio_definitions (id, name, description) VALUES
  (1, '默认组合', 'CI seed portfolio');

INSERT OR IGNORE INTO source_events (title, source, snippet, query, related_security_code, related_security_name, fetched_at, created_at) VALUES
  ('Market update', 'websearch', 'Markets moved...', 'AAPL market update', 'AAPL', 'Apple Inc.', '2026-06-18 10:00:00', '2026-06-18 10:00:00');
