package requestbudget

import (
	"context"
	"fmt"
	"time"
)

func Attempt(ctx context.Context, remaining int) (context.Context, context.CancelFunc, time.Duration) {
	if remaining < 1 {
		remaining = 1
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		child, cancel := context.WithCancel(ctx)
		return child, cancel, 0
	}
	budget := time.Until(deadline) / time.Duration(remaining)
	child, cancel := context.WithTimeout(ctx, budget)
	return child, cancel, budget
}

func CheckWait(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && delay > 0 && delay >= time.Until(deadline) {
		return fmt.Errorf("retry delay exceeds the remaining request budget: %w", context.DeadlineExceeded)
	}
	return nil
}

func Wait(ctx context.Context, delay time.Duration) error {
	if err := CheckWait(ctx, delay); err != nil {
		return err
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
