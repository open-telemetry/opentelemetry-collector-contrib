// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ratelimit

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

func TestRateLimit(t *testing.T) {
	cfg := NewRateLimitConfig("my_rate_limit")
	cfg.OutputIDs = []string{"fake"}
	cfg.Burst = 1
	cfg.Rate = 100

	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	op := ops[0]

	fake := testutil.NewFakeOutput(t)

	err = op.SetOutputs([]operator.Operator{fake})
	require.NoError(t, err)

	err = op.Start()
	defer op.Stop()
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, ok := <-fake.Received
			if !ok {
				return
			}
		}
	}()

	// Warm up
	for i := 0; i < 100; i++ {
		err := op.Process(context.Background(), entry.New())
		require.NoError(t, err)
	}

	// Measure
	start := time.Now()
	for i := 0; i < 500; i++ {
		err := op.Process(context.Background(), entry.New())
		require.NoError(t, err)
	}
	elapsed := time.Since(start)

	close(fake.Received)
	wg.Wait()

	if runtime.GOOS == "darwin" {
		t.Log("Using a wider acceptable range on darwin because of slow CI servers")
		require.InEpsilon(t, elapsed.Nanoseconds(), 5*time.Second.Nanoseconds(), 0.6)
	} else {
		require.InEpsilon(t, elapsed.Nanoseconds(), 5*time.Second.Nanoseconds(), 0.4)
	}
}
