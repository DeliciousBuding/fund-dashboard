package jobs

import (
	"context"
	"testing"
	"time"
)

func TestSleepContextHonorsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := sleepContext(ctx, 2*time.Second)
	if err == nil {
		t.Fatal("expected cancel error")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("blocked too long: %v", time.Since(start))
	}
}

func TestSleepContextCompletes(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	if err := sleepContext(ctx, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatal("returned too early")
	}
}
