package util

import (
	"context"
	"time"
)

// RunOnInterval the given fn once every interval, starting at the moment the
// function is called.  Returns a function that can be called to stop running
// the function.
func RunOnInterval(ctx context.Context, fn func(), interval time.Duration) {
	timer := time.NewTicker(interval)

	go func() {
		fn()

		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				fn()
			}
		}
	}()
}
