package jobs

import "testing"

// TestSchedulerClaimCodeIsStable pins the durable crawl_log keys. These strings
// are persisted: renaming one silently abandons the previous claim row, so a
// redeploy on the same day would run the job a second time. Every job uses the
// same "__sched_" + name rule, which is why schedulerClaimCode has no per-job
// branches - this test is what keeps it that way.
func TestSchedulerClaimCodeIsStable(t *testing.T) {
	cases := map[string]string{
		"startup_refresh": "__sched_startup_refresh",
		"price_dca":       "__sched_price_dca",
		"holdings":        "__sched_holdings",
		"wal":             "__sched_wal",
		"dca_backfill":    "__sched_dca_backfill",
	}
	for job, want := range cases {
		if got := schedulerClaimCode(job); got != want {
			t.Errorf("schedulerClaimCode(%q) = %q, want %q", job, got, want)
		}
	}
}
