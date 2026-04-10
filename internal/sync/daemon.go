package sync

import (
	"context"
	"time"
)

func RunDaemonLoop(ctx context.Context, interval time.Duration, fn func(context.Context) error) error {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	if err := fn(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				return err
			}
		}
	}
}
