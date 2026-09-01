package dialect

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// TestPostgresHasColumnRestrictsToPublicSchema pins the information_schema
// qualifier without a live PostgreSQL server: HasColumn must only see columns
// in the public schema, matching ListUserTables' schemaname = 'public' filter.
func TestPostgresHasColumnRestrictsToPublicSchema(t *testing.T) {
	conn := &capturingPGConn{
		queryFn: func(query string) ([]string, [][]driver.Value, error) {
			return []string{"exists"}, [][]driver.Value{{true}}, nil
		},
	}
	db := sql.OpenDB(&capturingPGConnector{conn: conn})
	t.Cleanup(func() { _ = db.Close() })

	found, err := New(NamePostgres, db).HasColumn(context.Background(), "nav_history", "unit_nav")
	if err != nil {
		t.Fatalf("HasColumn: %v", err)
	}
	if !found {
		t.Fatal("HasColumn = false, want true")
	}

	queries := conn.querySnapshot()
	if len(queries) != 1 {
		t.Fatalf("HasColumn issued %d queries, want 1: %v", len(queries), queries)
	}
	query := strings.ToLower(queries[0])
	if !strings.Contains(query, "table_schema = 'public'") {
		t.Fatalf("HasColumn query does not restrict table_schema to public: %q", queries[0])
	}
	if !strings.Contains(query, "information_schema.columns") {
		t.Fatalf("HasColumn query does not target information_schema.columns: %q", queries[0])
	}
}

// capturingPGConn implements just enough of database/sql/driver to capture the
// single-column query issued by Postgres.HasColumn.
type capturingPGConn struct {
	mu      sync.Mutex
	queries []string
	queryFn func(query string) ([]string, [][]driver.Value, error)
}

func (c *capturingPGConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("capturingPGConn: Prepare not expected")
}

func (c *capturingPGConn) Close() error { return nil }

func (c *capturingPGConn) Begin() (driver.Tx, error) {
	return nil, errors.New("capturingPGConn: Begin not expected")
}

func (c *capturingPGConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.mu.Lock()
	c.queries = append(c.queries, query)
	c.mu.Unlock()
	if c.queryFn == nil {
		return nil, errors.New("capturingPGConn: unexpected query")
	}
	cols, rows, err := c.queryFn(query)
	if err != nil {
		return nil, err
	}
	return &capturingPGRows{cols: cols, rows: rows}, nil
}

func (c *capturingPGConn) querySnapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.queries...)
}

type capturingPGRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *capturingPGRows) Columns() []string { return r.cols }
func (r *capturingPGRows) Close() error      { return nil }

func (r *capturingPGRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

type capturingPGConnector struct{ conn *capturingPGConn }

func (f *capturingPGConnector) Connect(context.Context) (driver.Conn, error) { return f.conn, nil }
func (f *capturingPGConnector) Driver() driver.Driver                        { return capturingPGDriver{} }

type capturingPGDriver struct{}

func (capturingPGDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("capturingPGDriver: Open not used")
}
