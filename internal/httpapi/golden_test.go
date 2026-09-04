package httpapi

// golden_test.go — 契约双向门禁的 Go 侧。
//
// 对每个契约域（packages/contracts/schemas/*.ts）选代表性端点，起 httptest
// router + 种子库，把真实 HTTP 响应落盘为金样本：
//
//	packages/contracts/testdata/golden/<域名>__<端点名>.json
//
// packages/contracts/golden.test.ts 读同一批文件用 zod schema parse——Go 或
// zod 任何一侧漂移都会红。
//
// 确定性：响应中随运行环境变化的字段（生成时间戳、uptime、go 版本、会话
// 时间戳等）在落盘/对比前统一按 goldenScrubStringKeys / goldenScrubNumberKeys
// 替换为哨兵值；两条路径走同一 scrub + canonical marshal，两次生成逐字节一致。
// 种子数据一律用固定日期（需要"相对新鲜"语义的场景用 date('now','-N day')
// 种子，使 stale天数 恒为 N）。
//
// 重新生成：GOLDEN_UPDATE=1 go test ./internal/httpapi -run TestGolden
// 默认模式逐字节对比，不一致即 FAIL 并提示用 GOLDEN_UPDATE=1 更新。

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

const goldenScrubSentinel = "__SCRUBBED__"

// goldenScrubStringKeys — 值随时钟/运行环境滚动的字符串字段（生成时间戳、
// 报告 ID、Go 版本、相对日期派生值等）。仅当值确实是 string 时替换。
var goldenScrubStringKeys = map[string]bool{
	"generated_at":     true, // harness / source brief / report（含 report.sections 内层）
	"checked_at":       true, // alerts
	"as_of":            true, // alerts item / report（相对日期派生）
	"report_id":        true, // rpt-<id>-<timestamp>
	"go_version":       true, // 随工具链变化
	"id_prefix":        true, // 会话令牌哈希前缀
	"timestamp":        true, // integrity 报告生成时刻
	"last_transaction": true, // freshness：MAX(trade_time)（相对日期种子）
	"last_nav_date":    true, // freshness：MAX(date)
	"last_nav":         true, // freshness：stale_securities[].last_nav
}

// goldenScrubNumberKeys — 值随时钟滚动的数值字段（unix 秒、uptime、相对天数）。
// 仅当值确实是 number 时替换；字符串实例（如种子固定的 created_at）保持原样。
var goldenScrubNumberKeys = map[string]bool{
	"uptime_sec":         true, // system status 进程 uptime
	"ts":                 true, // audit / auth 事件时间戳
	"created_at":         true, // auth SessionInfo（unix 秒；字符串实例不命中）
	"expires_at":         true,
	"last_seen_at":       true,
	"session_expires_at": true, // auth status
	"stale_nav_days":     true, // portfolio summary：随今天滚动的天数
}

// goldenScrub 递归把滚动字段替换为哨兵值，其余结构原样保留。
func goldenScrub(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			if goldenScrubStringKeys[key] {
				if _, isString := val.(string); isString {
					out[key] = goldenScrubSentinel
					continue
				}
			}
			if goldenScrubNumberKeys[key] {
				if _, isNumber := val.(float64); isNumber {
					out[key] = float64(0)
					continue
				}
			}
			out[key] = goldenScrub(val)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, val := range typed {
			out[i] = goldenScrub(val)
		}
		return out
	default:
		return value
	}
}

// goldenFakeJobStatus — 固定任务快照（同 TestSystemJobsReportSchedulerRuntime），
// 使 /api/system/jobs 金样本不依赖真实调度器时钟。
func goldenFakeJobStatus() []jobs.JobStatus {
	return []jobs.JobStatus{
		{Name: "price_dca", Schedule: "daily 20:00 CST", LastRun: 123456, NextRun: 789012},
		{Name: "wal", Schedule: "daily 03:00 CST", LastRun: 0, NextRun: 456789},
	}
}

// goldenCoreFixtureExtra — 在共享 portfolio fixture 之上补齐 SPA 扩展列与表；
// 时间列全部显式种子为固定值（金样本确定性），不使用 datetime('now')。
var goldenCoreFixtureExtra = []string{
	`ALTER TABLE fund_details ADD COLUMN currency TEXT DEFAULT ''`,
	`ALTER TABLE fund_details ADD COLUMN exchange TEXT DEFAULT ''`,
	`ALTER TABLE fund_details ADD COLUMN source TEXT DEFAULT ''`,
	`ALTER TABLE transactions ADD COLUMN anomaly TEXT`,
	`ALTER TABLE transactions ADD COLUMN portfolio_id INTEGER DEFAULT 1`,
	`CREATE TABLE portfolio_definitions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`INSERT INTO portfolio_definitions (id, name, description) VALUES (1, 'default', 'Default portfolio')`,
	`CREATE TABLE dca_plans (
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
	)`,
	`INSERT INTO dca_plans (id, fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, active, created_at, updated_at)
		VALUES (1, '019173', '纳斯达克100指数(QDII)C', 25, 'weekday', '1,3,5', '定投买入', 1, '2026-06-01', 1, '2026-06-01 08:00:00', '2026-06-01 08:00:00')`,
	// agent 审计行：created_at 固定在过去，保证 merged audit 时间线顺序稳定。
	`CREATE TABLE agent_audit_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id TEXT NOT NULL, caller TEXT NOT NULL, tool TEXT NOT NULL,
		event_type TEXT NOT NULL, status TEXT NOT NULL, scope TEXT NOT NULL,
		permission TEXT NOT NULL, risk_level TEXT NOT NULL,
		redacted_args_json TEXT NOT NULL, result_summary_json TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`INSERT INTO agent_audit_events (request_id, caller, tool, event_type, status, scope,
		permission, risk_level, redacted_args_json, result_summary_json, created_at)
		VALUES ('req-golden-1', 'cli', 'portfolio.summary', 'port_read', 'result', 'read', 'allowed', 'low', '{}', '{}', '2020-01-01T00:00:00Z')`,
}

// goldenAlertsFixture — 告警域种子：相对日期种子使 stale 天数恒定；
// 无 dca_plans 表（CheckAlerts 对缺表跳过 dca_day 告警，避免按星期漂移）。
var goldenAlertsFixture = []string{
	`CREATE TABLE fund_details (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT ''
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
	`CREATE TABLE nav_history (
		fund_code TEXT,
		date TEXT,
		unit_nav REAL,
		daily_change_pct REAL DEFAULT 0,
		security_type TEXT DEFAULT 'fund'
	)`,
	`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
		('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund', 'CN'),
		('AAPL', 'Apple Inc.', '科技股', 'stock', 'US')`,
	`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
		('019173', '纳斯达克100指数(QDII)C', 100, -120, 1.5, 150, 30, 25, 'fund', 1),
		('AAPL', 'Apple Inc.', 2, -300, 190, 380, 80, 26.67, 'stock', 1)`,
	// 相对日期种子带 '+8 hours'：SQLite date('now') 是 UTC，而 stale 天数按
	// Asia/Shanghai 计算；先 +8h 再回退 N 天，使 CST 语义下恒为 N 天前。
	`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES
		('019173', date('now', '+8 hours', '-2 day'), 1.5, 6.5, 'fund'),
		('AAPL', date('now', '+8 hours', '-10 day'), 190, 0.5, 'stock')`,
}

// goldenFreshnessFixture — freshness 域种子（相对日期使 stale_days 恒为 10，
// 缺 NAV / 观察名单各一只，见 admin_freshness_test.go 同款布局）。
var goldenFreshnessFixture = []string{
	`CREATE TABLE fund_details (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT ''
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
	`CREATE TABLE nav_history (
		fund_code TEXT,
		date TEXT,
		unit_nav REAL,
		daily_change_pct REAL DEFAULT 0,
		security_type TEXT DEFAULT 'fund'
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
	`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
		('HELD1', 'Held Missing NAV', '股票型', 'fund', 'CN'),
		('WATCH1', 'Watch Missing NAV', 'ETF', 'stock', 'SH'),
		('STALE1', 'Stale Fund', '指数型', 'fund', 'CN'),
		('FRESH1', 'Fresh Fund', '指数型', 'fund', 'CN')`,
	`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
		('HELD1', 'Held Missing NAV', 10, -10, 1, 10, 0, 0, 'fund', 1),
		('STALE1', 'Stale Fund', 10, -10, 1, 10, 0, 0, 'fund', 1),
		('FRESH1', 'Fresh Fund', 10, -10, 1, 10, 0, 0, 'fund', 1)`,
	// freshness 的 stale_days 走 SQL julianday('now')（UTC 基线），纯 UTC 相对
	// 日期种子经 CAST 截断后恒为 10/0；alerts fixture 的 Go 侧 CST 计算则用
	// '+8 hours' 种子——两个 fixture 各自匹配自己的计算时区。
	`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES
		('STALE1', date('now', '-10 day'), 1.0, 0, 'fund'),
		('FRESH1', date('now'), 1.0, 0, 'fund')`,
	`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
		VALUES ('TX-GOLDEN', datetime('now', '-3 day'), date('now', '-2 day'), '用户买入', 'buy', 'FRESH1', 'Fresh Fund', 10, 10, 0, -10, 10, 1)`,
}

// goldenEmit 请求端点、scrub、canonical marshal 后落盘/对比金样本。
func goldenEmit(t *testing.T, update bool, dir string, router http.Handler, method, path, file string, body any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body for %s: %v", file, err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.HasPrefix(path, "/api/admin/") {
		req.Header.Set("Authorization", "Bearer "+testAdminKey)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d, want 200; body=%s", method, path, res.Code, res.Body.String())
	}
	var decoded any
	if err := json.Unmarshal(res.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode %s response: %v; body=%s", file, err, res.Body.String())
	}
	payload, err := json.MarshalIndent(goldenScrub(decoded), "", "  ")
	if err != nil {
		t.Fatalf("marshal golden %s: %v", file, err)
	}
	payload = append(payload, '\n')

	target := filepath.Join(dir, file)
	if update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(target, payload, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", file, err)
		}
		return
	}
	want, err := os.ReadFile(target)
	if err != nil {
		t.Errorf("golden %s missing (%v); run GOLDEN_UPDATE=1 go test ./internal/httpapi -run TestGolden to generate", file, err)
		return
	}
	if !bytes.Equal(want, payload) {
		t.Errorf("golden mismatch for %s: Go wire shape drifted from the committed golden.\n"+
			"If the Go change is intended, regenerate with GOLDEN_UPDATE=1 go test ./internal/httpapi -run TestGolden\n"+
			"and re-run `node --test \"packages/contracts/**/*.test.ts\"` to re-validate the zod side.\n"+
			"--- committed golden ---\n%s\n--- current wire ---\n%s",
			file, want, payload)
	}
}

func TestGolden(t *testing.T) {
	update := os.Getenv("GOLDEN_UPDATE") == "1"
	dir := filepath.Join("..", "..", "packages", "contracts", "testdata", "golden")

	// ── core：funds / portfolio / analysis / harness / dca / reports / system / auth ──
	coreDB := testutil.OpenTempDBWithSchema(t,
		append(append([]string(nil), portfolioHTTPFixtureStatements...), goldenCoreFixtureExtra...))
	defer coreDB.Close()

	authSvc := newTestAuthService(t, coreDB)
	token := loginTestUser(t, authSvc)
	// 固定审计行：service 层直写（handler 层事件由 auth 端点测试覆盖），
	// ts 随时钟滚动 → 金样本 scrub 为 0，顺序由 (ts DESC, id DESC) 稳定。
	authSvc.RecordAuthEvent(context.Background(), "login_ok", "127.0.0.1", "test-agent", "")
	authSvc.RecordAuthEvent(context.Background(), "setup", "127.0.0.1", "test-agent", "")

	router := injectSession(NewRouter(testCfg(),
		WithDB(coreDB), WithAuth(authSvc), WithDBDriver("sqlite"), WithJobStatus(goldenFakeJobStatus)), token)

	goldenEmit(t, update, dir, router, http.MethodGet, "/api/funds", "funds__list.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/funds/019173", "funds__detail.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/funds/019173/nav", "funds__nav_history.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/transactions", "funds__transactions.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/funds/019173/dca?base=30&mode=nav_deviation", "funds__dca_compute.json", nil)

	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio", "portfolio__summary.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio/timeline", "portfolio__timeline.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio/portfolios", "portfolio__portfolios.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio/allocation", "portfolio__allocation.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio/penetration", "portfolio__penetration.json", nil)

	goldenEmit(t, update, dir, router, http.MethodGet, "/api/funds/019173/xirr", "analysis__fund_xirr.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/funds/019173/drawdown", "analysis__fund_drawdown.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio/xirr", "analysis__portfolio_xirr.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/analysis/compare?codes=019173", "analysis__compare.json", nil)

	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio/harness", "harness__snapshot.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio/source-brief", "harness__source_brief.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/portfolio/source-events", "harness__source_events.json", nil)

	goldenEmit(t, update, dir, router, http.MethodPost, "/api/reports", "reports__generate.json",
		map[string]any{"title": "月度报告", "as_of": "2026-06-18"})

	goldenEmit(t, update, dir, router, http.MethodGet, "/api/dca/plans", "dca__plans.json", nil)

	goldenEmit(t, update, dir, router, http.MethodGet, "/api/system/status", "system__status.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/system/jobs", "system__jobs.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/system/agent", "system__agent.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/system/audit", "system__audit.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/system/integrity", "system__integrity.json", nil)

	goldenEmit(t, update, dir, router, http.MethodGet, "/api/auth/status", "auth__status.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/auth/sessions", "auth__sessions.json", nil)
	goldenEmit(t, update, dir, router, http.MethodGet, "/api/auth/events", "auth__events.json", nil)

	// ── alerts：独立种子（无 dca_plans 表 + 相对日期） ──
	alertsDB := testutil.OpenTempDBWithSchema(t, goldenAlertsFixture)
	defer alertsDB.Close()
	alertsRouter := newAuthedRouter(t, testCfg(), alertsDB)
	goldenEmit(t, update, dir, alertsRouter, http.MethodGet,
		"/api/alerts?price_change_pct=5&drawdown_pct=10&stale_days=4", "alerts__check.json", nil)

	// ── admin：freshness（相对日期种子，stale_days 恒为 10） ──
	freshnessDB := testutil.OpenTempDBWithSchema(t, goldenFreshnessFixture)
	defer freshnessDB.Close()
	freshnessRouter := NewRouter(testCfg(), WithDB(freshnessDB))
	goldenEmit(t, update, dir, freshnessRouter, http.MethodGet, "/api/admin/freshness", "admin__freshness.json", nil)

	// ── market / stocks：indices 缓存种子（2099 时间戳 → 永远新鲜，不触发外呼） ──
	marketDB := testutil.OpenTempDBWithSchema(t, marketHTTPFixtureStatements)
	defer marketDB.Close()
	marketRouter := newAuthedRouter(t, testCfg(), marketDB)

	// 汇率：本地桩上游 + 清空进程内缓存，保证每次都走同一 fetch 路径。
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"chart": {
				"result": [{
					"meta": {
						"regularMarketPrice": 7.2513,
						"regularMarketTime": 1783439100
					}
				}]
			}
		}`))
	}))
	defer upstream.Close()
	t.Setenv("FUND_EXCHANGE_RATE_ENDPOINT", upstream.URL)
	portfoliosvc.ResetExchangeRateCache()

	goldenEmit(t, update, dir, marketRouter, http.MethodGet, "/api/market/indices", "market__indices.json", nil)
	goldenEmit(t, update, dir, marketRouter, http.MethodGet, "/api/market/index/GSPC", "market__index_live.json", nil)
	goldenEmit(t, update, dir, marketRouter, http.MethodGet, "/api/market/exchange-rate", "market__exchange_rate.json", nil)
	goldenEmit(t, update, dir, marketRouter, http.MethodGet, "/api/stocks/AAPL?range=1y&include_history=true", "stocks__aapl.json", nil)
}
