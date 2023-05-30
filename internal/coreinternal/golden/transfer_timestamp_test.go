package golden

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestCopyTimestamp(t *testing.T) {
	testCases := []struct {
		desc                string
		expectedMetricsFile string
		actualMetricsFile   string
		testMetricsFile     string
	}{
		{
			desc:                "Expected and Actual metrics file are identical except for the timestamps",
			expectedMetricsFile: filepath.Join("testdata", "time-stamp-difference", "expected.yaml"),
			actualMetricsFile:   filepath.Join("testdata", "time-stamp-difference", "actual.yaml"),
			testMetricsFile:     filepath.Join("testdata", "time-stamp-difference", "test_comparison.yaml"),
		},
		{
			desc:                "Actual Metrics has a datapoint removed",
			expectedMetricsFile: filepath.Join("testdata", "time-stamp-remove-datapoint", "expected.yaml"),
			actualMetricsFile:   filepath.Join("testdata", "time-stamp-remove-datapoint", "actual.yaml"),
			testMetricsFile:     filepath.Join("testdata", "time-stamp-remove-datapoint", "test_comparison.yaml"),
		},
		{
			desc:                "Actual Metrics has a datapoint added",
			expectedMetricsFile: filepath.Join("testdata", "time-stamp-add-datapoint", "expected.yaml"),
			actualMetricsFile:   filepath.Join("testdata", "time-stamp-add-datapoint", "actual.yaml"),
			testMetricsFile:     filepath.Join("testdata", "time-stamp-add-datapoint", "test_comparison.yaml"),
		},
		{
			desc:                "Actual Metrics has a metric removed",
			expectedMetricsFile: filepath.Join("testdata", "time-stamp-remove-metric", "expected.yaml"),
			actualMetricsFile:   filepath.Join("testdata", "time-stamp-remove-metric", "actual.yaml"),
			testMetricsFile:     filepath.Join("testdata", "time-stamp-remove-metric", "test_comparison.yaml"),
		},
		{
			desc:                "Actual Metrics has a metric added",
			expectedMetricsFile: filepath.Join("testdata", "time-stamp-add-metric", "expected.yaml"),
			actualMetricsFile:   filepath.Join("testdata", "time-stamp-add-metric", "actual.yaml"),
			testMetricsFile:     filepath.Join("testdata", "time-stamp-add-metric", "test_comparison.yaml"),
		},
	}

	for _, tc := range testCases {
		expected, err := ReadMetrics(tc.expectedMetricsFile)
		require.NoError(t, err)
		actual, err := ReadMetrics(tc.actualMetricsFile)
		require.NoError(t, err)
		testComparison, err := ReadMetrics(tc.testMetricsFile)
		require.NoError(t, err)

		CopyTimeStamps(expected, actual)

		assert.NoError(t, pmetrictest.CompareMetrics(actual, testComparison,
			pmetrictest.IgnoreResourceMetricsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}
}
