// Package chinatime is the single source of truth for China-market timezone
// definitions. The fund NAV calendar follows the CN A-share / fund industry
// convention (UTC+8); code that renders or computes calendar days for that
// market must use Loc instead of a host-local zone.
package chinatime

import "time"

// Loc is the China-market calendar location (+08:00). It prefers the IANA
// "Asia/Shanghai" zone when tzdata is available and falls back to a fixed
// UTC+8 zone so slim containers without zoneinfo keep identical behavior.
var Loc = loadLoc()

func loadLoc() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*3600)
}
