// Package textutil holds tiny shared text helpers used across service and API
// layers. It exists so free-text bounding logic has exactly one home instead
// of one copy per package (#243/#244/#232 lineage).
package textutil

import "strings"

// Clamp trims surrounding whitespace and bounds s to at most max runes
// (rune-safe cut for CJK labels). max <= 0 means "trim only".
func Clamp(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}
