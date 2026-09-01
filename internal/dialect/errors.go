package dialect

import "strings"

// IsMissingTableError reports whether err indicates that a referenced table is
// absent from the schema, across the supported drivers:
//
//   - SQLite: "no such table"
//   - PostgreSQL: "does not exist"
//   - legacy drivers: "undefined_table"
//
// The check inspects the final error message, so errors wrapped with
// fmt.Errorf("%w", ...) are still recognized.
func IsMissingTableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "undefined_table")
}

// IsDuplicateColumnError reports whether err indicates an ALTER TABLE ADD
// COLUMN hit a column that already exists, across the supported drivers:
//
//   - SQLite: "duplicate column name" (older builds: "duplicate column")
//   - PostgreSQL: "already exists" / SQLSTATE 42701 (duplicate_column)
//
// Callers must only apply it to ADD COLUMN failures: "already exists" alone
// is ambiguous outside that statement class. The check inspects the final
// error message, so errors wrapped with fmt.Errorf("%w", ...) are recognized.
func IsDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "duplicate_column") ||
		strings.Contains(msg, "42701")
}
