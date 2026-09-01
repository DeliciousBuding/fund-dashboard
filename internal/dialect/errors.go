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
