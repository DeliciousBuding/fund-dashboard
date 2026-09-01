package httpapi

import (
	"errors"
	"fmt"
	"testing"
)

// TestIsMissingTableErr pins the dialect-independent schema-absence check used
// by /api/system/audit: SQLite, PostgreSQL, and legacy drivers must all be
// tolerated when agent_audit_events does not exist, while real faults are not.
func TestIsMissingTableErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"sqlite", errors.New("SQL logic error: no such table: agent_audit_events (1)"), true},
		{"postgres", errors.New(`pq: relation "agent_audit_events" does not exist`), true},
		{"postgres wrapped", fmt.Errorf("list agent audit events: %w", errors.New(`pq: relation "agent_audit_events" does not exist`)), true},
		{"legacy", errors.New("undefined_table"), true},
		{"locked", errors.New("database is locked"), false},
		{"constraint", errors.New("UNIQUE constraint failed"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isMissingTableErr(tc.err); got != tc.want {
				t.Fatalf("isMissingTableErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
