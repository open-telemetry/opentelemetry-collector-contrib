// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type replayTimer struct {
	prev      pcommon.Timestamp
	throttle  float64 // set to 1.0 to replay at same speed, 2.0 for half speed, etc.
	sleepFunc func(ctx context.Context, d time.Duration) error
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
