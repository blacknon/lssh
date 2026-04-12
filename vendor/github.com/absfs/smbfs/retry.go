package smbfs

import (
	"context"
	"time"
)

// RetryPolicy defines retry behavior for operations.
type RetryPolicy struct {
	MaxAttempts  int           // Maximum number of attempts (default: 3)
	InitialDelay time.Duration // Initial delay between retries (default: 100ms)
	MaxDelay     time.Duration // Maximum delay between retries (default: 5s)
	Multiplier   float64       // Backoff multiplier (default: 2.0)
}

// defaultRetryPolicy is the default retry policy.
var defaultRetryPolicy = &RetryPolicy{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     5 * time.Second,
	Multiplier:   2.0,
}

// withRetry executes an operation with retry logic using exponential backoff.
func (fsys *FileSystem) withRetry(ctx context.Context, operation func() error) error {
	policy := fsys.config.RetryPolicy
	if policy == nil {
		policy = defaultRetryPolicy
	}

	// If MaxAttempts is 0 or 1, don't retry
	if policy.MaxAttempts <= 1 {
		return operation()
	}

	var lastErr error
	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Attempt operation
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry if error is not retryable
		if !isRetryable(err) {
			return err
		}

		// Don't retry on last attempt
		if attempt == policy.MaxAttempts {
			break
		}

		// Log retry attempt if logger is configured
		if fsys.config.Logger != nil {
			fsys.config.Logger.Printf("Operation failed (attempt %d/%d), retrying in %v: %v",
				attempt, policy.MaxAttempts, delay, err)
		}

		// Exponential backoff with jitter
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		// Calculate next delay
		delay = time.Duration(float64(delay) * policy.Multiplier)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}

	return lastErr
}
