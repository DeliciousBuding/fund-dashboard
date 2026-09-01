package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

// ── structural pins ────────────────────────────────────────────────────────
//
// EnsurePGSchema emits PostgreSQL-only DDL (SERIAL, TIMESTAMPTZ, ::type casts,
// GENERATED ... AS IDENTITY, pg_get_constraintdef, DO $$ blocks), so it cannot
// be executed against the SQLite test target used by the rest of this package.
// These tests pin every property of pgSchemaStatements that does not need a
// live server:
//
//   - every statement is idempotent (CREATE ... IF NOT EXISTS) and
//     creation-only (no ALTER/DROP/UPDATE/INSERT/DELETE), so running the
//     bootstrap twice is safe by construction;
//   - required tables, columns, primary keys and indexes are present;
//   - every index references a table defined in the same list;
//   - the SQLite schema mirrors the PG schema (no cross-dialect drift).
//
// The execution and migration control flow of EnsurePGSchema is covered by the
// in-memory fake driver below. Live PostgreSQL execution remains out of unit
// test reach by design.
var (
	pgTableNameRe = regexp.MustCompile(`^CREATE TABLE IF NOT EXISTS\s+([a-z_][a-z0-9_]*)\s*\(`)
	pgIndexStmtRe = regexp.MustCompile(`^CREATE INDEX IF NOT EXISTS\s+([a-z_][a-z0-9_]*)\s+ON\s+([a-z_][a-z0-9_]*)`)
	pgPKRe        = regexp.MustCompile(`PRIMARY KEY\s*\(([^)]*)\)`)
)

type schemaTable struct {
	name    string
	columns []string
	pk      []string
}

type schemaIndex struct {
	name  string
	table string
}

func parseSchemaTable(t *testing.T, stmt string) schemaTable {
	t.Helper()
	nameMatch := pgTableNameRe.FindStringSubmatch(stmt)
	if nameMatch == nil {
		t.Fatalf("table statement not idempotent or unparsable: %q", stmt)
	}
	open := strings.Index(stmt, "(")
	close := strings.LastIndex(stmt, ")")
	if open < 0 || close <= open {
		t.Fatalf("malformed CREATE TABLE statement: %q", stmt)
	}
	tbl := schemaTable{name: nameMatch[1]}
	for _, line := range strings.Split(stmt[open+1:close], "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		upper := strings.ToUpper(fields[0])
		if upper == "PRIMARY" || upper == "FOREIGN" || upper == "UNIQUE" ||
			upper == "CHECK" || upper == "CONSTRAINT" {
			continue
		}
		tbl.columns = append(tbl.columns, fields[0])
		// Inline PK forms (e.g. "id INTEGER PRIMARY KEY AUTOINCREMENT").
		if strings.Contains(strings.ToUpper(line), "PRIMARY KEY") &&
			!strings.Contains(strings.ToUpper(line), "PRIMARY KEY (") {
			tbl.pk = append(tbl.pk, fields[0])
		}
	}
	if m := pgPKRe.FindStringSubmatch(stmt); m != nil {
		for _, c := range strings.Split(m[1], ",") {
			tbl.pk = append(tbl.pk, strings.TrimSpace(c))
		}
	}
	return tbl
}

func parseSchemaIndex(t *testing.T, stmt string) schemaIndex {
	t.Helper()
	m := pgIndexStmtRe.FindStringSubmatch(stmt)
	if m == nil {
		t.Fatalf("index statement not idempotent or unparsable: %q", stmt)
	}
	return schemaIndex{name: m[1], table: m[2]}
}

func parseSchemaStatements(t *testing.T, stmts []string) ([]schemaTable, []schemaIndex) {
	t.Helper()
	var tables []schemaTable
	var indexes []schemaIndex
	for _, stmt := range stmts {
		switch {
		case strings.HasPrefix(stmt, "CREATE TABLE"):
			if !strings.HasPrefix(stmt, "CREATE TABLE IF NOT EXISTS ") {
				t.Fatalf("non-idempotent CREATE TABLE: %q", stmt)
			}
			tables = append(tables, parseSchemaTable(t, stmt))
		case strings.HasPrefix(stmt, "CREATE INDEX"):
			if !strings.HasPrefix(stmt, "CREATE INDEX IF NOT EXISTS ") {
				t.Fatalf("non-idempotent CREATE INDEX: %q", stmt)
			}
			indexes = append(indexes, parseSchemaIndex(t, stmt))
		default:
			t.Fatalf("unexpected schema statement: %q", stmt)
		}
	}
	return tables, indexes
}

func sortedEqual(a, b []string) bool {
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return reflect.DeepEqual(x, y)
}

func TestPGSchemaStatementsAreIdempotentCreationOnly(t *testing.T) {
	tables, indexes := parseSchemaStatements(t, pgSchemaStatements)

	seenTables := make(map[string]bool, len(tables))
	for _, tb := range tables {
		if seenTables[tb.name] {
			t.Fatalf("duplicate table definition: %s", tb.name)
		}
		seenTables[tb.name] = true
		// ant_transactions_* are intentionally PK-less staging tables; every
		// other table must declare a primary key.
		if len(tb.pk) == 0 && tb.name != "ant_transactions_normalized" && tb.name != "ant_transactions_raw" {
			t.Fatalf("table %s has no PRIMARY KEY", tb.name)
		}
	}
	seenIndexes := make(map[string]bool, len(indexes))
	for _, ix := range indexes {
		if seenIndexes[ix.name] {
			t.Fatalf("duplicate index definition: %s", ix.name)
		}
		seenIndexes[ix.name] = true
		if !seenTables[ix.table] {
			t.Fatalf("index %s references unknown table %q", ix.name, ix.table)
		}
	}

	// The bootstrap list must never contain stateful or destructive SQL: that
	// is what makes repeated execution side-effect free.
	for _, stmt := range pgSchemaStatements {
		upper := strings.ToUpper(stmt)
		for _, bad := range []string{"ALTER TABLE", "DROP ", "DELETE ", "INSERT ", "UPDATE "} {
			if strings.Contains(upper, bad) {
				t.Fatalf("pgSchemaStatements contains %q: %q", bad, stmt)
			}
		}
	}
}

func TestPGSchemaDefinesRequiredTablesAndColumns(t *testing.T) {
	tables, _ := parseSchemaStatements(t, pgSchemaStatements)
	byName := make(map[string]schemaTable, len(tables))
	for _, tb := range tables {
		byName[tb.name] = tb
	}

	required := map[string][]string{
		"transactions":        {"order_id", "fund_code", "confirm_amount", "settlement_days", "portfolio_id", "anomaly"},
		"portfolio_snapshot":  {"fund_code", "portfolio_id", "held_shares", "total_cost"},
		"nav_history":         {"date", "fund_code", "unit_nav"},
		"dca_plans":           {"fund_code", "active", "portfolio_id", "amount", "frequency"},
		"dca_plan_executions": {"plan_id", "trade_date", "status"},
		"fund_holdings":       {"fund_code", "stock_code", "report_date", "weight_pct"},
		"source_events":       {"id", "title", "url", "fetched_at"},
		"stock_kline_cache":   {"code", "period", "date", "close"},
		"stock_profile":       {"code", "name", "market"},
		"stock_realtime":      {"code", "price", "updated_at"},
		"summary_by_fund":     {"fund_code", "total_shares", "total_cost"},
		"sector_map":          {"stock_code", "market", "sector"},
		"auth_credentials":    {"password_hash", "updated_at"},
		"auth_sessions":       {"expires_at", "last_seen_at"},
		"auth_events":         {"ts", "event"},
	}
	for name, cols := range required {
		tb, ok := byName[name]
		if !ok {
			t.Fatalf("required table %q missing from pgSchemaStatements", name)
		}
		have := make(map[string]bool, len(tb.columns))
		for _, c := range tb.columns {
			have[c] = true
		}
		for _, c := range cols {
			if !have[c] {
				t.Fatalf("table %s is missing column %s", name, c)
			}
		}
	}

	pkWant := map[string][]string{
		"transactions":       {"seq"},
		"portfolio_snapshot": {"fund_code", "portfolio_id"},
		"nav_history":        {"date", "fund_code"},
		"fund_holdings":      {"fund_code", "stock_code", "report_date"},
		"stock_kline_cache":  {"code", "period", "date"},
		"sector_map":         {"stock_code", "market"},
	}
	for name, want := range pkWant {
		if got := byName[name].pk; !reflect.DeepEqual(got, want) {
			t.Fatalf("table %s primary key = %v, want %v", name, got, want)
		}
	}
}

func TestPGSchemaDefinesRequiredIndexes(t *testing.T) {
	_, indexes := parseSchemaStatements(t, pgSchemaStatements)
	have := make(map[string]schemaIndex, len(indexes))
	for _, ix := range indexes {
		have[ix.name] = ix
	}
	required := []string{
		"idx_transactions_fund_code",
		"idx_transactions_trade_time",
		"idx_nav_history_fund",
		"idx_nav_history_date",
		"idx_nav_history_fund_date",
		"idx_dca_plans_active_portfolio",
		"idx_dca_exec_plan_date",
		"idx_fund_holdings_fund",
		"idx_portfolio_snapshot_portfolio",
		"idx_dca_plans_fund",
		"idx_agent_confirmations_tool",
		"idx_agent_audit_events_tool",
		"idx_auth_sessions_expires",
		"idx_auth_events_ts",
	}
	for _, name := range required {
		if _, ok := have[name]; !ok {
			t.Fatalf("required index %q missing from pgSchemaStatements", name)
		}
	}
	// The transactions and portfolio hot paths rely on these exact shapes.
	if got := have["idx_transactions_fund_code"].table; got != "transactions" {
		t.Fatalf("idx_transactions_fund_code table = %q, want transactions", got)
	}
	if got := have["idx_portfolio_snapshot_portfolio"].table; got != "portfolio_snapshot" {
		t.Fatalf("idx_portfolio_snapshot_portfolio table = %q, want portfolio_snapshot", got)
	}
}

func TestPGAndSQLiteSchemaParity(t *testing.T) {
	pgTables, _ := parseSchemaStatements(t, pgSchemaStatements)
	sqliteTables, _ := parseSchemaStatements(t, sqliteSchemaTables)

	pgByName := make(map[string]schemaTable, len(pgTables))
	for _, tb := range pgTables {
		pgByName[tb.name] = tb
	}
	// Every SQLite table must be a field-complete mirror of the PG table: the
	// schema_sqlite.go contract says PG is the field-complete reference and
	// PG-only columns are added to the SQLite DDL. PG additionally owns the
	// agent/auth tables, which have their own SQLite bootstrap elsewhere.
	for _, sb := range sqliteTables {
		pb, ok := pgByName[sb.name]
		if !ok {
			t.Fatalf("sqlite table %q missing from pgSchemaStatements (drift)", sb.name)
		}
		if !sortedEqual(sb.columns, pb.columns) {
			t.Fatalf("table %s columns drifted:\n  sqlite: %v\n  pg:     %v", sb.name, sb.columns, pb.columns)
		}
		if !sortedEqual(sb.pk, pb.pk) {
			t.Fatalf("table %s primary key drifted:\n  sqlite: %v\n  pg:     %v", sb.name, sb.pk, pb.pk)
		}
	}
}

// ── EnsurePGSchema control flow (in-memory fake driver) ────────────────────

// fakePGConn implements the database/sql driver interfaces consumed by
// EnsurePGSchema so its execution and migration control flow can be tested
// without a live PostgreSQL server.
type fakePGConn struct {
	mu       sync.Mutex
	execLog  []string
	queryLog []string
	execFn   func(query string) (driver.Result, error)
	queryFn  func(query string) ([]string, [][]driver.Value, error)
}

func (c *fakePGConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fakePGConn: Prepare not expected")
}

func (c *fakePGConn) Close() error { return nil }

func (c *fakePGConn) Begin() (driver.Tx, error) {
	return nil, errors.New("fakePGConn: Begin not expected")
}

func (c *fakePGConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.mu.Lock()
	c.execLog = append(c.execLog, query)
	c.mu.Unlock()
	if c.execFn != nil {
		return c.execFn(query)
	}
	return driver.RowsAffected(0), nil
}

func (c *fakePGConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.mu.Lock()
	c.queryLog = append(c.queryLog, query)
	c.mu.Unlock()
	if c.queryFn == nil {
		return nil, fmt.Errorf("fakePGConn: unexpected query %q", query)
	}
	cols, rows, err := c.queryFn(query)
	if err != nil {
		return nil, err
	}
	return &fakePGRows{cols: cols, rows: rows}, nil
}

func (c *fakePGConn) execSnapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.execLog...)
}

type fakePGRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *fakePGRows) Columns() []string { return r.cols }
func (r *fakePGRows) Close() error      { return nil }
func (r *fakePGRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

type fakePGConnector struct{ conn *fakePGConn }

func (f *fakePGConnector) Connect(context.Context) (driver.Conn, error) { return f.conn, nil }
func (f *fakePGConnector) Driver() driver.Driver                        { return fakePGDriver{} }

type fakePGDriver struct{}

func (fakePGDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("fakePGDriver: Open not used")
}

func newFakePGDB(t *testing.T, execFn func(string) (driver.Result, error), queryFn func(string) ([]string, [][]driver.Value, error)) (*sql.DB, *fakePGConn) {
	t.Helper()
	conn := &fakePGConn{execFn: execFn, queryFn: queryFn}
	dbi := sql.OpenDB(&fakePGConnector{conn: conn})
	dbi.SetMaxOpenConns(1)
	dbi.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = dbi.Close() })
	return dbi, conn
}

// condefQuery simulates the two catalog probes issued by
// migratePortfolioSnapshotPK: the pg_get_constraintdef lookup and the
// duplicate-group count.
func condefQuery(condef string, dups int64) func(string) ([]string, [][]driver.Value, error) {
	return func(q string) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "pg_get_constraintdef"):
			return []string{"condef"}, [][]driver.Value{{condef}}, nil
		case strings.Contains(q, "GROUP BY fund_code"):
			return []string{"dups"}, [][]driver.Value{{dups}}, nil
		default:
			return nil, nil, fmt.Errorf("unexpected query: %q", q)
		}
	}
}

func indexOfSubstr(items []string, needle string) int {
	for i, s := range items {
		if strings.Contains(s, needle) {
			return i
		}
	}
	return -1
}

func TestEnsurePGSchemaRunsTwiceWithoutSideEffects(t *testing.T) {
	ctx := context.Background()
	dbi, conn := newFakePGDB(t, nil, condefQuery("PRIMARY KEY (fund_code, portfolio_id)", 0))

	for i := 0; i < 2; i++ {
		if err := EnsurePGSchema(ctx, dbi); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}

	execs := conn.execSnapshot()
	// Each run issues pgSchemaStatements + the best-effort unique index. The
	// migration probe sees an already-composite PK and must not run any
	// UPDATE/DO mutation, so the second run's sequence must be identical to
	// the first (true no-side-effect idempotency at the orchestration level).
	perRun := len(pgSchemaStatements) + 1
	if len(execs) != 2*perRun {
		t.Fatalf("exec count = %d, want %d (two identical runs)", len(execs), 2*perRun)
	}
	first, second := execs[:perRun], execs[perRun:]
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("second run differs from first run\n first: %q\nsecond: %q", first, second)
	}
	for i, stmt := range pgSchemaStatements {
		if first[i] != stmt {
			t.Fatalf("statement %d differs\n got: %q\nwant: %q", i, first[i], stmt)
		}
	}
	last := first[perRun-1]
	if !strings.Contains(last, "idx_transactions_order_fund_unique") ||
		!strings.Contains(last, "(order_id, fund_code)") {
		t.Fatalf("unique index statement missing or changed: %q", last)
	}
	for _, q := range first {
		if strings.Contains(q, "UPDATE portfolio_snapshot") || strings.Contains(q, "ADD PRIMARY KEY") {
			t.Fatalf("unexpected migration side effect: %q", q)
		}
	}
}

func TestEnsurePGSchemaMigratesLegacyPortfolioSnapshotPK(t *testing.T) {
	ctx := context.Background()
	dbi, conn := newFakePGDB(t, nil, condefQuery("PRIMARY KEY (fund_code)", 0))
	if err := EnsurePGSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsurePGSchema: %v", err)
	}
	execs := conn.execSnapshot()
	updateAt := indexOfSubstr(execs, "UPDATE portfolio_snapshot SET portfolio_id = 1")
	doAt := indexOfSubstr(execs, "ADD PRIMARY KEY (fund_code, portfolio_id)")
	if updateAt < 0 {
		t.Fatalf("legacy null portfolio_id fill UPDATE missing: %v", execs)
	}
	if doAt < 0 {
		t.Fatalf("composite PK migration DO block missing: %v", execs)
	}
	if updateAt > doAt {
		t.Fatalf("fill UPDATE must run before the PK DDL block: update=%d do=%d", updateAt, doAt)
	}
}

func TestEnsurePGSchemaSkipsMigrationOnDuplicateGroups(t *testing.T) {
	ctx := context.Background()
	dbi, conn := newFakePGDB(t, nil, condefQuery("PRIMARY KEY (fund_code)", 3))
	if err := EnsurePGSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsurePGSchema: %v", err)
	}
	execs := conn.execSnapshot()
	if indexOfSubstr(execs, "UPDATE portfolio_snapshot SET portfolio_id = 1") < 0 {
		t.Fatalf("fill UPDATE missing: %v", execs)
	}
	if indexOfSubstr(execs, "ADD PRIMARY KEY") >= 0 {
		t.Fatalf("PK DDL ran despite duplicate groups: %v", execs)
	}
}

func TestEnsurePGSchemaMigrationProbeTolerance(t *testing.T) {
	ctx := context.Background()

	// Hard catalog error: migration must degrade to a warning, not fail boot,
	// and must not touch the table.
	failing := func(q string) ([]string, [][]driver.Value, error) {
		return nil, nil, errors.New("catalog probe exploded")
	}
	dbi, conn := newFakePGDB(t, nil, failing)
	if err := EnsurePGSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsurePGSchema with failing probe: %v", err)
	}
	execs := conn.execSnapshot()
	if indexOfSubstr(execs, "UPDATE portfolio_snapshot") >= 0 || indexOfSubstr(execs, "ADD PRIMARY KEY") >= 0 {
		t.Fatalf("migration ran despite failing probe: %v", execs)
	}

	// No PK constraint found (probe returns no rows): the migration path is
	// entered and, with no duplicate groups, the composite PK is added.
	noRows := func(q string) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "pg_get_constraintdef"):
			return []string{"condef"}, nil, nil
		case strings.Contains(q, "GROUP BY fund_code"):
			return []string{"dups"}, [][]driver.Value{{int64(0)}}, nil
		default:
			return nil, nil, fmt.Errorf("unexpected query: %q", q)
		}
	}
	dbi2, conn2 := newFakePGDB(t, nil, noRows)
	if err := EnsurePGSchema(ctx, dbi2); err != nil {
		t.Fatalf("EnsurePGSchema with no PK rows: %v", err)
	}
	execs2 := conn2.execSnapshot()
	if indexOfSubstr(execs2, "UPDATE portfolio_snapshot SET portfolio_id = 1") < 0 {
		t.Fatalf("fill UPDATE missing when no PK found: %v", execs2)
	}
	if indexOfSubstr(execs2, "ADD PRIMARY KEY (fund_code, portfolio_id)") < 0 {
		t.Fatalf("composite PK DDL missing when no PK found: %v", execs2)
	}
}
func TestEnsurePGSchemaFailsOnStatementError(t *testing.T) {
	ctx := context.Background()
	execFn := func(q string) (driver.Result, error) {
		if strings.Contains(q, "nav_history") {
			return nil, errors.New("boom")
		}
		return driver.RowsAffected(0), nil
	}
	dbi, conn := newFakePGDB(t, execFn, condefQuery("PRIMARY KEY (fund_code, portfolio_id)", 0))
	err := EnsurePGSchema(ctx, dbi)
	if err == nil {
		t.Fatal("EnsurePGSchema: want error for failing statement")
	}
	if !strings.Contains(err.Error(), "pg schema stmt") {
		t.Fatalf("error = %q, want mention of the failing statement index", err)
	}
	// Execution must stop at the failing statement.
	execs := conn.execSnapshot()
	want := 0
	for i, stmt := range pgSchemaStatements {
		if strings.Contains(stmt, "nav_history") {
			want = i + 1
			break
		}
	}
	if len(execs) != want {
		t.Fatalf("exec count = %d, want stop at %d", len(execs), want)
	}
}

func TestEnsurePGSchemaUniqueIndexFailureDoesNotFailBoot(t *testing.T) {
	ctx := context.Background()
	execFn := func(q string) (driver.Result, error) {
		if strings.Contains(q, "idx_transactions_order_fund_unique") {
			return nil, errors.New("duplicate legacy rows")
		}
		return driver.RowsAffected(0), nil
	}
	dbi, conn := newFakePGDB(t, execFn, condefQuery("PRIMARY KEY (fund_code, portfolio_id)", 0))
	if err := EnsurePGSchema(ctx, dbi); err != nil {
		t.Fatalf("unique index failure must not fail boot: %v", err)
	}
	execs := conn.execSnapshot()
	if len(execs) != len(pgSchemaStatements)+1 {
		t.Fatalf("exec count = %d, want %d", len(execs), len(pgSchemaStatements)+1)
	}
}
