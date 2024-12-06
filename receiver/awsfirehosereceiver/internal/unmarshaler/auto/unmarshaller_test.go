package auto

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog/compression"
)

func TestType(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewExample())
	require.Equal(t, TypeStr, unmarshaler.Type())
}

// Unmarshall cloudwatch metrics and logs
func TestUnmarshal_CW(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())

	testCases := map[string]struct {
		dir                  string
		filename             string
		logResourceCount     int
		logRecordCount       int
		metricResourceCount  int
		metricCount          int
		metricDataPointCount int
		err                  error
	}{
		"cwlog:WithMultipleRecords": {
			dir:              "cwlog",
			filename:         "multiple_records",
			logResourceCount: 1,
			logRecordCount:   2,
		},
		"cwlog:WithSingleRecord": {
			dir:              "cwlog",
			filename:         "single_record",
			logResourceCount: 1,
			logRecordCount:   1,
		},
		"cwlog:WithInvalidRecords": {
			dir:      "cwlog",
			filename: "invalid_records",
			err:      errInvalidRecords,
		},
		"cwlog:WithSomeInvalidRecords": {
			dir:              "cwlog",
			filename:         "some_invalid_records",
			logResourceCount: 1,
			logRecordCount:   2,
		},
		"cwlog:WithMultipleResources": {
			dir:              "cwlog",
			filename:         "multiple_resources",
			logResourceCount: 3,
			logRecordCount:   6,
		},
		"cwmetric:WithMultipleRecords": {
			dir:                  "cwmetricstream",
			filename:             "multiple_records",
			metricResourceCount:  6,
			metricCount:          33,
			metricDataPointCount: 127,
		},
		"cwmetric:WithSingleRecord": {
			dir:                  "cwmetricstream",
			filename:             "single_record",
			metricResourceCount:  1,
			metricCount:          1,
			metricDataPointCount: 1,
		},
		"cwmetric:WithInvalidRecords": {
			dir:      "cwmetricstream",
			filename: "invalid_records",
			err:      errInvalidRecords,
		},
		"cwmetric:WithSomeInvalidRecords": {
			dir:                  "cwmetricstream",
			filename:             "some_invalid_records",
			metricResourceCount:  5,
			metricCount:          35,
			metricDataPointCount: 88,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join("..", testCase.dir, "testdata", testCase.filename))
			require.NoError(t, err)

			compressedRecord, err := compression.Zip(record)
			require.NoError(t, err)
			records := [][]byte{compressedRecord}

			metrics, logs, err := unmarshaler.Unmarshal(records)
			require.Equal(t, testCase.err, err)

			require.Equal(t, testCase.logResourceCount, logs.ResourceLogs().Len())
			require.Equal(t, testCase.logRecordCount, logs.LogRecordCount())

			require.Equal(t, testCase.metricResourceCount, metrics.ResourceMetrics().Len())
			require.Equal(t, testCase.metricDataPointCount, metrics.DataPointCount())
			require.Equal(t, testCase.metricCount, metrics.MetricCount())
		})
	}

}

func createMetricRecord() []byte {
	er := pmetricotlp.NewExportRequest()
	rsm := er.Metrics().ResourceMetrics().AppendEmpty()
	sm := rsm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	sm.SetName("TestMetric")
	dp := sm.SetEmptySummary().DataPoints().AppendEmpty()
	dp.SetCount(1)
	dp.SetSum(1)
	qv := dp.QuantileValues()
	min := qv.AppendEmpty()
	min.SetQuantile(0)
	min.SetValue(0)
	max := qv.AppendEmpty()
	max.SetQuantile(1)
	max.SetValue(1)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	temp, _ := er.MarshalProto()
	record := proto.EncodeVarint(uint64(len(temp)))
	record = append(record, temp...)
	return record
}
