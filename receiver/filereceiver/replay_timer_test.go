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

package filereceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestReplayTimer(t *testing.T) {
	s := &fakeSleeper{}
	timer := &replayTimer{
		throttle:  0.5,
		sleepFunc: s.fakeSleep,
	}
	firstMetricTime := time.Date(2020, time.January, 1, 1, 0, 0, 0, time.UTC)
	err := timer.wait(context.Background(), pcommon.NewTimestampFromTime(firstMetricTime))
	require.NoError(t, err)
	secondMetricTime := firstMetricTime.Add(time.Second * 10)
	err = timer.wait(context.Background(), pcommon.NewTimestampFromTime(secondMetricTime))
	require.NoError(t, err)
	err = timer.wait(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, s.durations, []time.Duration{0, time.Second * 5})
}

type fakeSleeper struct {
	durations []time.Duration
}

func (t *fakeSleeper) fakeSleep(_ context.Context, d time.Duration) error {
	t.durations = append(t.durations, d)
	return nil
}
