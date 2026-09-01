package jobs

import (
	"os"
	"strings"
	"testing"
)

// nav_schema.go must use the dialect single source for duplicate-column
// detection; inline substring matching would drift between dialects again
// (round-8 follow-up of the round-6 missing-table convergence).
func TestNavSchemaUsesDialectDuplicateColumnSource(t *testing.T) {
	raw, err := os.ReadFile("nav_schema.go")
	if err != nil {
		t.Fatalf("read nav_schema.go: %v", err)
	}
	src := string(raw)
	if strings.Contains(src, "isDuplicateColumnErr") {
		t.Fatal("nav_schema.go still defines/uses local isDuplicateColumnErr")
	}
	if !strings.Contains(src, "dialect.IsDuplicateColumnError") {
		t.Fatal("nav_schema.go must call dialect.IsDuplicateColumnError")
	}
}
