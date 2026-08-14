package portfolio

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/repository/sqlitedb"
	_ "modernc.org/sqlite"
)

func TestServiceGetSummaryMatchesCurrentPortfolioSemantics(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	service := NewService(db)
	summary, err := service.GetSummary(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}
	if summary == nil {
		t.Fatalf("GetSummary returned nil, want summary")
	}

	if summary.TotalTx != 5 {
		t.Fatalf("TotalTx = %d, want 5", summary.TotalTx)
	}
	if summary.UniqueFunds != 2 {
		t.Fatalf("UniqueFunds = %d, want 2", summary.UniqueFunds)
	}
	if summary.UniqueStocks != 0 {
		t.Fatalf("UniqueStocks = %d, want 0", summary.UniqueStocks)
	}
	if summary.HeldFunds != 2 {
		t.Fatalf("HeldFunds = %d, want 2", summary.HeldFunds)
	}
	if summary.TotalBuy != 500 {
		t.Fatalf("TotalBuy = %.2f, want 500", summary.TotalBuy)
	}
	if summary.TotalSell != 80 {
		t.Fatalf("TotalSell = %.2f, want 80", summary.TotalSell)
	}
	if summary.TotalFee != 0.87 {
		t.Fatalf("TotalFee = %.2f, want 0.87", summary.TotalFee)
	}
	if summary.UnrealizedPNL != 80.03 {
		t.Fatalf("UnrealizedPNL = %.2f, want 80.03", summary.UnrealizedPNL)
	}
	if summary.InvestedCost != 420 {
		t.Fatalf("InvestedCost = %.2f, want 420", summary.InvestedCost)
	}
	if summary.CurrentValue != 500.03 {
		t.Fatalf("CurrentValue = %.2f, want 500.03", summary.CurrentValue)
	}
	if summary.PNLPct != 19.05 {
		t.Fatalf("PNLPct = %.2f, want 19.05", summary.PNLPct)
	}
	if summary.TopGainer == nil || summary.TopGainer.Code != "019173" {
		t.Fatalf("TopGainer = %#v, want 019173", summary.TopGainer)
	}
	// both positions profitable — TopLoser should be nil
	if summary.TopLoser != nil {
		t.Fatalf("TopLoser = %#v, want nil (no losing held position)", summary.TopLoser)
	}
	if summary.AutoTx != 3 || summary.ManualTx != 2 {
		t.Fatalf("AutoTx/ManualTx = %d/%d, want 3/2", summary.AutoTx, summary.ManualTx)
	}
	if summary.AutoAmount != 300 || summary.ManualAmount != 200 {
		t.Fatalf("AutoAmount/ManualAmount = %.2f/%.2f, want 300/200", summary.AutoAmount, summary.ManualAmount)
	}
	if summary.FirstTrade != "2024-06-01" || summary.LastTrade != "2025-03-20" {
		t.Fatalf("FirstTrade/LastTrade = %q/%q, want 2024-06-01/2025-03-20", summary.FirstTrade, summary.LastTrade)
	}
	if summary.LastNAVDate == nil || *summary.LastNAVDate != "2025-05-01" {
		t.Fatalf("LastNAVDate = %v, want 2025-05-01", summary.LastNAVDate)
	}
	if summary.SettlementDistribution["2"] != 4 || summary.SettlementDistribution["3"] != 1 {
		t.Fatalf("SettlementDistribution = %#v, want 2:4 and 3:1", summary.SettlementDistribution)
	}
	if summary.TradeTypeBreakdown["定投买入"] != 3 || summary.TradeTypeBreakdown["用户买入"] != 1 || summary.TradeTypeBreakdown["用户卖出"] != 1 {
		t.Fatalf("TradeTypeBreakdown = %#v, want current TS counts", summary.TradeTypeBreakdown)
	}
	if len(summary.BySecurityType) != 1 {
		t.Fatalf("BySecurityType length = %d, want 1", len(summary.BySecurityType))
	}
	if summary.BySecurityType[0] != (SecurityTypeBalance{SecurityType: "fund", Count: 2, TotalValue: 500.03, TotalPNL: 80.03}) {
		t.Fatalf("BySecurityType[0] = %#v, want fund balance", summary.BySecurityType[0])
	}
}

func TestServiceGetSummaryScopesSnapshotFactsByPortfolio(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type) VALUES ('AAPL', 'Apple Inc.', 'Technology', 'stock');
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
		VALUES ('AAPL', 'Apple Inc.', 2, -300, 190, 380, 80, 26.67, 'stock', 2);
	`); err != nil {
		t.Fatalf("seed portfolio 2: %v", err)
	}

	service := NewService(db)
	summary, err := service.GetSummary(context.Background(), 2)
	if err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}

	if summary.HeldFunds != 1 {
		t.Fatalf("HeldFunds = %d, want 1 for portfolio 2", summary.HeldFunds)
	}
	if summary.UniqueStocks != 1 {
		t.Fatalf("UniqueStocks = %d, want 1 for portfolio 2", summary.UniqueStocks)
	}
	if summary.UnrealizedPNL != 80 {
		t.Fatalf("UnrealizedPNL = %.2f, want portfolio 2 snapshot pnl 80", summary.UnrealizedPNL)
	}
	if len(summary.BySecurityType) != 1 || summary.BySecurityType[0].SecurityType != "stock" {
		t.Fatalf("BySecurityType = %#v, want stock-only portfolio 2 balance", summary.BySecurityType)
	}
}

func TestServiceListPortfolioDefinitionsReturnsConfiguredPortfolios(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE portfolio_definitions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO portfolio_definitions (id, name, description) VALUES
			(2, 'satellite', 'Satellite sleeve'),
			(1, 'default', 'Default portfolio');
	`); err != nil {
		t.Fatalf("seed portfolio definitions: %v", err)
	}

	service := NewService(db)
	portfolios, err := service.ListPortfolioDefinitions(context.Background())
	if err != nil {
		t.Fatalf("ListPortfolioDefinitions returned error: %v", err)
	}
	if len(portfolios) != 2 {
		t.Fatalf("portfolios length = %d, want 2: %#v", len(portfolios), portfolios)
	}
	if portfolios[0] != (PortfolioDefinition{ID: 1, Name: "default", Description: "Default portfolio"}) ||
		portfolios[1] != (PortfolioDefinition{ID: 2, Name: "satellite", Description: "Satellite sleeve"}) {
		t.Fatalf("portfolios = %#v, want id-sorted definitions", portfolios)
	}
}

func TestServiceListDCAPlansFiltersActivePlansByPortfolio(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE dca_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fund_code TEXT NOT NULL,
			fund_name TEXT,
			amount REAL NOT NULL,
			frequency TEXT NOT NULL DEFAULT 'weekday',
			weekday_mask TEXT NOT NULL DEFAULT '1,2,3,4,5',
			trade_type TEXT NOT NULL DEFAULT '定投买入',
			portfolio_id INTEGER NOT NULL DEFAULT 1,
			start_date TEXT NOT NULL,
			end_date TEXT,
			active INTEGER NOT NULL DEFAULT 1,
			source TEXT NOT NULL DEFAULT 'manual',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO dca_plans (id, fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, end_date, active, source, created_at, updated_at)
		VALUES
			(1, '018439', '国泰纳斯达克100ETF联接C', 30, 'weekday', '1,3,5', '定投买入', 1, '2026-06-01', NULL, 1, 'manual', '2026-06-01 09:00:00', '2026-06-02 09:00:00'),
			(2, '019173', '纳斯达克100指数(QDII)C', 20, 'weekday', '2,4', '定投买入', 1, '2026-06-03', '2026-12-31', 0, 'mcp', '2026-06-03 09:00:00', '2026-06-04 09:00:00'),
			(3, '000001', 'Other Portfolio', 10, 'weekday', '1,2,3,4,5', '定投买入', 2, '2026-06-05', NULL, 1, 'manual', '2026-06-05 09:00:00', '2026-06-06 09:00:00');
	`); err != nil {
		t.Fatalf("seed dca plans: %v", err)
	}

	service := NewService(db)
	plans, err := service.ListDCAPlans(context.Background(), ListDCAPlansOptions{
		ActiveOnly:  true,
		PortfolioID: 1,
	})
	if err != nil {
		t.Fatalf("ListDCAPlans returned error: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans length = %d, want one active portfolio 1 plan: %#v", len(plans), plans)
	}
	plan := plans[0]
	if plan.ID != 1 ||
		plan.FundCode != "018439" ||
		plan.FundName == nil ||
		*plan.FundName != "国泰纳斯达克100ETF联接C" ||
		plan.Amount != 30 ||
		plan.WeekdayMask != "1,3,5" ||
		plan.Active != 1 ||
		plan.EndDate != nil ||
		plan.Source != "manual" {
		t.Fatalf("plan = %#v, want active portfolio 1 DCA rule facts", plan)
	}
}

func openSummaryFixture(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "fund.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("open sqlite fixture: %v", err)
	}

	for _, stmt := range summaryFixtureStatements {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			db.Close()
			t.Fatalf("exec fixture statement %q: %v", stmt, err)
		}
	}
	return db
}

var summaryFixtureStatements = []string{
	`CREATE TABLE fund_details (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT ''
	)`,
	`CREATE TABLE transactions (
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
	)`,
	`CREATE TABLE nav_history (
		fund_code TEXT,
		date TEXT,
		unit_nav REAL,
		daily_change_pct REAL DEFAULT 0,
		security_type TEXT DEFAULT 'fund'
	)`,
	`CREATE TABLE portfolio_snapshot (
			fund_code TEXT NOT NULL,
		fund_name TEXT,
		held_shares REAL,
		total_cost REAL,
		latest_nav REAL,
		current_value REAL,
		unrealized_pnl REAL,
		pnl_pct REAL,
		security_type TEXT DEFAULT 'fund',
			portfolio_id INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
	`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type) VALUES
		('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund'),
		('018439', '国泰纳斯达克100ETF联接C', 'QDII-ETF联接', 'fund')`,
	`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
		VALUES
		('TX001', '2024-06-01T09:00:00Z', '2024-06-02', '定投买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 100, 85.47, 0.15, -100, 85.47, 2),
		('TX002', '2024-07-15T09:00:00Z', '2024-07-16', '用户买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 200, 166.67, 0.30, -200, 166.67, 2),
		('TX003', '2025-03-20T09:00:00Z', '2025-03-21', '用户卖出', 'sell', '019173', '纳斯达克100指数(QDII)C', 80, 55.17, 0.12, 80, -55.17, 3),
		('TX006', '2024-06-01T09:00:00Z', '2024-06-02', '定投买入', 'buy', '018439', '国泰纳斯达克100ETF联接C', 100, 90.91, 0.15, -100, 90.91, 2),
		('TX007', '2024-09-01T09:00:00Z', '2024-09-02', '定投买入', 'buy', '018439', '国泰纳斯达克100ETF联接C', 100, 78.74, 0.15, -100, 78.74, 2)`,
	`INSERT INTO nav_history (date, fund_code, unit_nav) VALUES
		('2024-06-01', '019173', 1.1700),
		('2024-08-01', '019173', 1.2500),
		('2025-01-15', '019173', 1.5800),
		('2025-03-15', '019173', 1.1000),
		('2025-05-01', '019173', 1.3500),
		('2024-06-01', '018439', 1.1000),
		('2025-01-05', '018439', 1.3800)`,
	`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
		('019173', '纳斯达克100指数(QDII)C', 196.97, 220, 1.3500, 265.91, 45.91, 20.87, 'fund', 1),
		('018439', '国泰纳斯达克100ETF联接C', 169.65, 200, 1.3800, 234.12, 34.12, 17.06, 'fund', 1)`,
}
