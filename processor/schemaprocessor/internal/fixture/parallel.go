// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fixture

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/race"
)

// ParallelCompute will run the function concurrently by syncing the start of each operation
// and waiting for each to finish. The returned result from the function
// is checked to see if it is not null.
func ParallelCompute(tb testing.TB, concurrency int, fn func() error) {
	tb.Helper()

	require.NotNil(tb, fn, "Must have a valid function")

	var (
		start = make(chan struct{})
		wg    sync.WaitGroup
	)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			<-start
			assert.NoError(tb, fn(), "Must not error when executing function")
		}()
	}
	close(start)

	wg.Wait()
}

// ParallelRaceCompute checks to see if the race detector is running before
// testing the function concurrently
func ParallelRaceCompute(tb testing.TB, concurrency int, fn func() error) {
	tb.Helper()
	if !race.Enabled {
		tb.Skip(
			"This test requires the Race Detector to be enabled.",
			"Please run again with -race to run this test.",
		)
		return
	}
	ParallelCompute(tb, concurrency, fn)
}
