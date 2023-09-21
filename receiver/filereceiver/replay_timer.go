// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type replayTimer struct {
	sleepFunc func(ctx context.Context, d time.Duration) error
	prev      pcommon.Timestamp
	throttle  float64
}

func newReplayTimer(throttle float64) *replayTimer {
	return &replayTimer{
		throttle:  throttle,
		sleepFunc: sleepWithContext,
	}
}

func (t *replayTimer) wait(ctx context.Context, next pcommon.Timestamp) error {
	if next == 0 {
		return nil
	}
	var sleepDuration pcommon.Timestamp
	if t.prev > 0 {
		sleepDuration = pcommon.Timestamp(float64(next-t.prev) * t.throttle)
	}
	t.prev = next
	err := t.sleepFunc(ctx, time.Duration(sleepDuration))
	if err != nil {
		return fmt.Errorf("context cancelled while waiting for replay timer: %w", err)
	}
	return nil
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
