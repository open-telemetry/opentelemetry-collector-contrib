// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package winperfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCounterPath(t *testing.T) {
	testCases := []struct {
		name         string
		object       string
		instance     string
		counterName  string
		expectedPath string
	}{
		{
			name:         "basicPath",
			object:       "Memory",
			counterName:  "Committed Bytes",
			expectedPath: "\\Memory\\Committed Bytes",
		},
		{
			name:         "basicPathWithInstance",
			object:       "Web Service",
			instance:     "_Total",
			counterName:  "Current Connections",
			expectedPath: "\\Web Service(_Total)\\Current Connections",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			path := counterPath(test.object, test.instance, test.counterName)
			require.Equal(t, test.expectedPath, path)
		})
	}
}

// Test_Scraping_Wildcard tests that wildcard instances pull out values
func Test_Scraping_Wildcard(t *testing.T) {
	watcher, err := NewWatcher("LogicalDisk", "*", "Free Megabytes")
	require.NoError(t, err)

	values, err := watcher.ScrapeData()
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(values), 3)
}

func TestNewPerfCounter_InvalidPath(t *testing.T) {
	_, err := newPerfCounter("Invalid Counter Path", false)
	if assert.Error(t, err) {
		assert.Regexp(t, "^Unable to parse the counter path", err.Error())
	}
}

func TestNewPerfCounter(t *testing.T) {
	pc, err := newPerfCounter(`\Memory\Committed Bytes`, false)
	require.NoError(t, err, "Failed to create performance counter: %v", err)

	assert.NotNil(t, pc.query)
	assert.NotNil(t, pc.handle)

	// the first collection will return a zero value
	var vals []CounterValue
	vals, err = pc.query.GetFormattedCounterArrayDouble(pc.handle)
	require.NoError(t, err)
	assert.Equal(t, []CounterValue{{InstanceName: "", Value: 0}}, vals)

	err = pc.query.Close()
	require.NoError(t, err, "Failed to close initialized performance counter query: %v", err)
}

func TestNewPerfCounter_CollectOnStartup(t *testing.T) {
	pc, err := newPerfCounter(`\Memory\Committed Bytes`, true)
	require.NoError(t, err, "Failed to create performance counter: %v", err)

	assert.NotNil(t, pc.query)
	assert.NotNil(t, pc.handle)

	// since we collected on startup, the next collection will return a measured value
	var vals []CounterValue
	vals, err = pc.query.GetFormattedCounterArrayDouble(pc.handle)
	require.NoError(t, err)
	assert.Greater(t, vals[0].Value, float64(0))

	err = pc.query.Close()
	require.NoError(t, err, "Failed to close initialized performance counter query: %v", err)
}

func TestPerfCounter_Close(t *testing.T) {
	pc, err := newPerfCounter(`\Memory\Committed Bytes`, false)
	require.NoError(t, err)

	err = pc.Close()
	require.NoError(t, err, "Failed to close initialized performance counter query: %v", err)

	err = pc.Close()
	if assert.Error(t, err) {
		assert.Equal(t, "uninitialised query", err.Error())
	}
}

func TestPerfCounter_NonExistentInstance_NoError(t *testing.T) {
	pc, err := newPerfCounter(`\.NET CLR Memory(NonExistentInstance)\% Time in GC`, true)
	require.NoError(t, err)

	data, err := pc.ScrapeData()
	require.NoError(t, err)

	assert.Empty(t, data)
}

func TestPerfCounter_Reset(t *testing.T) {
	pc, err := newPerfCounter(`\Memory\Committed Bytes`, false)
	require.NoError(t, err)

	path, handle, query := pc.Path(), pc.handle, pc.query

	err = pc.Reset()

	// new query is different instance of same counter.
	require.NoError(t, err)
	assert.NotEqual(t, handle, pc.handle)
	assert.NotSame(t, query, pc.query)
	assert.Equal(t, path, pc.Path())

	err = query.Close() // previous query is closed
	if assert.Error(t, err) {
		assert.Equal(t, "uninitialised query", err.Error())
	}
}

func TestPerfCounter_ScrapeData(t *testing.T) {
	type testCase struct {
		name           string
		path           string
		assertExpected func(t *testing.T, data []CounterValue)
	}

	testCases := []testCase{
		{
			name: "no instances",
			path: `\Memory\Committed Bytes`,
			assertExpected: func(t *testing.T, data []CounterValue) {
				assert.Len(t, data, 1)
				assert.Empty(t, data[0].InstanceName)
			},
		},
		{
			name: "total instance",
			path: `\LogicalDisk(_Total)\Free Megabytes`,
			assertExpected: func(t *testing.T, data []CounterValue) {
				assert.Len(t, data, 1)
				assert.Empty(t, data[0].InstanceName)
			},
		},
		{
			name: "all instances",
			path: `\LogicalDisk(*)\Free Megabytes`,
			assertExpected: func(t *testing.T, data []CounterValue) {
				assert.GreaterOrEqual(t, len(data), 1)
				for _, d := range data {
					assert.NotEmpty(t, d.InstanceName)
				}
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			pc, err := newPerfCounter(test.path, false)
			require.NoError(t, err)

			data, err := pc.ScrapeData()
			require.NoError(t, err, "Failed to scrape data: %v", err)

			test.assertExpected(t, data)
		})
	}
}

func Test_InstanceNameIndexing(t *testing.T) {
	type testCase struct {
		name     string
		vals     []CounterValue
		expected []CounterValue
	}

	testCases := []testCase{
		{
			name: "Multiple distinct instances",
			vals: []CounterValue{
				{
					InstanceName: "A",
					Value:        1.0,
				},
				{
					InstanceName: "B",
					Value:        1.0,
				},
				{
					InstanceName: "C",
					Value:        1.0,
				},
			},
			expected: []CounterValue{
				{
					InstanceName: "A",
					Value:        1.0,
				},
				{
					InstanceName: "B",
					Value:        1.0,
				},
				{
					InstanceName: "C",
					Value:        1.0,
				},
			},
		},
		{
			name: "Single repeated instance name",
			vals: []CounterValue{
				{
					InstanceName: "A",
					Value:        1.0,
				},
				{
					InstanceName: "A",
					Value:        1.0,
				},
				{
					InstanceName: "A",
					Value:        1.0,
				},
			},
			expected: []CounterValue{
				{
					InstanceName: "A",
					Value:        1.0,
				},
				{
					InstanceName: "A#1",
					Value:        1.0,
				},
				{
					InstanceName: "A#2",
					Value:        1.0,
				},
			},
		},
		{
			name: "Multiple repeated instance name",
			vals: []CounterValue{
				{
					InstanceName: "A",
					Value:        1.0,
				},
				{
					InstanceName: "B",
					Value:        1.0,
				},
				{
					InstanceName: "A",
					Value:        1.0,
				},
				{
					InstanceName: "B",
					Value:        1.0,
				},
				{
					InstanceName: "B",
					Value:        1.0,
				},
				{
					InstanceName: "C",
					Value:        1.0,
				},
			},
			expected: []CounterValue{
				{
					InstanceName: "A",
					Value:        1.0,
				},
				{
					InstanceName: "B",
					Value:        1.0,
				},
				{
					InstanceName: "A#1",
					Value:        1.0,
				},
				{
					InstanceName: "B#1",
					Value:        1.0,
				},
				{
					InstanceName: "B#2",
					Value:        1.0,
				},
				{
					InstanceName: "C",
					Value:        1.0,
				},
			},
		},
	}

	for _, test := range testCases {
		actual := cleanupScrapedValues(test.vals)
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, actual)
		})
	}
}
