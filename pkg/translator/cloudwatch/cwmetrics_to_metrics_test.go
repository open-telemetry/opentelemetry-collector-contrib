package cloudwatch

import (
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"testing"
)

func TestUnmarshalMetrics(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		filename       string
		resourceCount  int
		metricCount    int
		datapointCount int
		err            error
	}{
		"WithMultipleRecords": {
			filename:       "multiple_records",
			resourceCount:  6,
			metricCount:    33,
			datapointCount: 127,
		},
		"WithSingleRecord": {
			filename:       "single_record",
			resourceCount:  1,
			metricCount:    1,
			datapointCount: 1,
		},
		"WithInvalidRecords": {
			filename: "invalid_records",
			err:      errInvalidMetricRecord,
		},
		"WithSomeInvalidRecords": {
			filename:       "some_invalid_records",
			resourceCount:  5,
			metricCount:    36,
			datapointCount: 88,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join("testdata/metric", testCase.filename))
			require.NoError(t, err)

			metrics, err := UnmarshalMetrics(record, zap.NewNop())

			require.Equal(t, testCase.err, err)
			require.Equal(t, testCase.resourceCount, metrics.ResourceMetrics().Len())
			require.Equal(t, testCase.metricCount, metrics.MetricCount())
			require.Equal(t, testCase.datapointCount, metrics.DataPointCount())
		})
	}
}
