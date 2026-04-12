package sync

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestHasPathPrefix(t *testing.T) {
	t.Parallel()

	if !hasPathPrefix("/dest/app/file.txt", "/dest/app", func(path string) string { return path }, "/") {
		t.Fatalf("expected child path to be inside scope")
	}
	if hasPathPrefix("/dest/app2/file.txt", "/dest/app", func(path string) string { return path }, "/") {
		t.Fatalf("expected sibling path to be outside scope")
	}
}

func TestRunBidirectionalLoopRepeatsServerCyclesInDaemonMode(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &Sync{Daemon: true, DaemonInterval: 10 * time.Millisecond}
	var count atomic.Int32

	s.runBidirectionalLoop(ctx, []string{"a", "b"}, func(context.Context, string) error {
		if count.Add(1) >= 4 {
			cancel()
		}
		return nil
	})

	if got := count.Load(); got < 4 {
		t.Fatalf("expected at least two daemon cycles across both servers, got %d calls", got)
	}
}
