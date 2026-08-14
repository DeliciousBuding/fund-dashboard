package jobs

import (
	"os"
	"strings"
	"testing"
)

func TestSchedulerJobTimeoutsWired(t *testing.T) {
	// Source-level guard: long crawls must not use bare context.Background without timeout (#248 / #268).
	raw, err := os.ReadFile("scheduler.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, "45 * time.Minute") && !strings.Contains(src, "45*time.Minute") {
		t.Fatal("expected 45m job timeouts")
	}
	if !strings.Contains(src, "2 * time.Minute") && !strings.Contains(src, "2*time.Minute") {
		t.Fatal("expected 2m indices timeout")
	}
	// weekday/startup/holdings should derive timeouts from root (jobContext) so Stop cancels them.
	if strings.Count(src, "jobContext(") < 3 {
		t.Fatalf("expected >=3 jobContext derivations, got %d", strings.Count(src, "jobContext("))
	}
	if !strings.Contains(src, "startupTimer") || !strings.Contains(src, "rootCancel") {
		t.Fatal("expected startupTimer + rootCancel for Stop cancel")
	}
}
