// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed

import (
	"testing"
	"time"
)

func Test_perWorkerTickDuration(t *testing.T) {
	for _, test := range []struct {
		name                                          string
		expectedTickDuration                          time.Duration
		dataItemsPerSecond, itemsPerBatch, numWorkers int
	}{
		// Because of the way perWorkerTickDuration calculates the tick interval using dataItemsPerSecond,
		// it is important to test its behavior with respect to a one-second Duration in particular because
		// certain combinations of configuration could previously cause a divide-by-zero panic.
		{
			name:                 "less than one second",
			expectedTickDuration: 100 * time.Millisecond,
			dataItemsPerSecond:   100,
			itemsPerBatch:        5,
			numWorkers:           2,
		},
		{
			name:                 "exactly one second",
			expectedTickDuration: time.Second,
			dataItemsPerSecond:   100,
			itemsPerBatch:        5,
			numWorkers:           20,
		},
		{
			name:                 "more than one second (would previously trigger divide-by-zero panic)",
			expectedTickDuration: 5 * time.Second,
			dataItemsPerSecond:   100,
			itemsPerBatch:        5,
			numWorkers:           100,
		},
		{
			name:                 "default batch size and worker count",
			expectedTickDuration: 8103727, // ~8.1ms
			dataItemsPerSecond:   1234,
			itemsPerBatch:        10,
			numWorkers:           1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			subject := &ProviderSender{
				options: LoadOptions{
					DataItemsPerSecond: test.dataItemsPerSecond,
					ItemsPerBatch:      test.itemsPerBatch,
				},
			}
			actual := subject.perWorkerTickDuration(test.numWorkers)
			if actual != test.expectedTickDuration {
				t.Errorf("got %v; want %v", actual, test.expectedTickDuration)
			}
		})
	}
}
