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

//go:build windows
// +build windows

package pdh

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/internal/third_party/telegraf/win_perf_counters"
)

func TestNewPerfCounter_InvalidPath(t *testing.T) {
	_, err := NewPerfCounter("Invalid Counter Path", false)
	if assert.Error(t, err) {
		assert.Regexp(t, "^Unable to parse the counter path", err.Error())
	}
}

func TestNewPerfCounter(t *testing.T) {
	pc, err := NewPerfCounter(`\Memory\Committed Bytes`, false)
	require.NoError(t, err, "Failed to create performance counter: %v", err)

	assert.NotNil(t, pc.query)
	assert.NotNil(t, pc.handle)

	// the first collection will return a zero value
	var vals []win_perf_counters.CounterValue
	vals, err = pc.query.GetFormattedCounterArrayDouble(pc.handle)
	require.NoError(t, err)
	assert.Equal(t, []win_perf_counters.CounterValue{{InstanceName: "", Value: 0}}, vals)

	err = pc.query.Close()
	require.NoError(t, err, "Failed to close initialized performance counter query: %v", err)
}

func TestNewPerfCounter_CollectOnStartup(t *testing.T) {
	pc, err := NewPerfCounter(`\Memory\Committed Bytes`, true)
	require.NoError(t, err, "Failed to create performance counter: %v", err)

	assert.NotNil(t, pc.query)
	assert.NotNil(t, pc.handle)

	// since we collected on startup, the next collection will return a measured value
	var vals []win_perf_counters.CounterValue
	vals, err = pc.query.GetFormattedCounterArrayDouble(pc.handle)
	require.NoError(t, err)
	assert.Greater(t, vals[0].Value, float64(0))

	err = pc.query.Close()
	require.NoError(t, err, "Failed to close initialized performance counter query: %v", err)
}

func TestPerfCounter_Close(t *testing.T) {
	pc, err := NewPerfCounter(`\Memory\Committed Bytes`, false)
	require.NoError(t, err)

	err = pc.Close()
	require.NoError(t, err, "Failed to close initialized performance counter query: %v", err)

	err = pc.Close()
	if assert.Error(t, err) {
		assert.Equal(t, "uninitialised query", err.Error())
	}
}

func TestPerfCounter_ScrapeData(t *testing.T) {
	type testCase struct {
		name           string
		path           string
		assertExpected func(t *testing.T, data []win_perf_counters.CounterValue)
	}

	testCases := []testCase{
		{
			name: "no instances",
			path: `\Memory\Committed Bytes`,
			assertExpected: func(t *testing.T, data []win_perf_counters.CounterValue) {
				assert.Len(t, data, 1)
				assert.Empty(t, data[0].InstanceName)
			},
		},
		{
			name: "total instance",
			path: `\LogicalDisk(_Total)\Free Megabytes`,
			assertExpected: func(t *testing.T, data []win_perf_counters.CounterValue) {
				assert.Equal(t, 1, len(data))
				assert.Empty(t, data[0].InstanceName)
			},
		},
		{
			name: "all instances",
			path: `\LogicalDisk(*)\Free Megabytes`,
			assertExpected: func(t *testing.T, data []win_perf_counters.CounterValue) {
				assert.GreaterOrEqual(t, len(data), 1)
				for _, d := range data {
					assert.NotEmpty(t, d.InstanceName)
				}
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			pc, err := NewPerfCounter(test.path, false)
			require.NoError(t, err)

			data, err := pc.ScrapeData()
			require.NoError(t, err, "Failed to scrape data: %v", err)

			test.assertExpected(t, data)
		})
	}
}
