package sync

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunDaemonLoopRepeatsUntilCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var count atomic.Int32
	err := RunDaemonLoop(ctx, 10*time.Millisecond, func(context.Context) error {
		if count.Add(1) >= 3 {
			cancel()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunDaemonLoop returned error: %v", err)
	}

	if got := count.Load(); got < 3 {
		t.Fatalf("expected loop to run at least 3 times, got %d", got)
	}
}
