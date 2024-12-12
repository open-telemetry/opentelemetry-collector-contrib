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

func TestUnmarshalMetrics_JSON(t *testing.T) {
	t.Parallel()

	unmarshaler := NewUnmarshaler(zap.NewNop())
	testCases := map[string]struct {
		dir                  string
		filename             string
		metricResourceCount  int
		metricCount          int
		metricDataPointCount int
		err                  error
	}{
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

			metrics, err := unmarshaler.UnmarshalMetrics("application/json", records)
			require.Equal(t, testCase.err, err)

			require.Equal(t, testCase.metricResourceCount, metrics.ResourceMetrics().Len())
			require.Equal(t, testCase.metricDataPointCount, metrics.DataPointCount())
			require.Equal(t, testCase.metricCount, metrics.MetricCount())
		})
	}
}

// Unmarshall cloudwatch metrics and logs
func TestUnmarshalLogs_JSON(t *testing.T) {
	t.Parallel()

	unmarshaler := NewUnmarshaler(zap.NewNop())
	testCases := map[string]struct {
		dir              string
		filename         string
		logResourceCount int
		logRecordCount   int
		err              error
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
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join("..", testCase.dir, "testdata", testCase.filename))
			require.NoError(t, err)

			compressedRecord, err := compression.Zip(record)
			require.NoError(t, err)
			records := [][]byte{compressedRecord}

			logs, err := unmarshaler.UnmarshalLogs("application/json", records)
			require.Equal(t, testCase.err, err)

			require.Equal(t, testCase.logResourceCount, logs.ResourceLogs().Len())
			require.Equal(t, testCase.logRecordCount, logs.LogRecordCount())
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

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	unmarshaler := NewUnmarshaler(zap.NewNop())
	testCases := map[string]struct {
		records            [][]byte
		wantResourceCount  int
		wantMetricCount    int
		wantDatapointCount int
		wantErr            error
	}{
		"WithSingleRecord": {
			records: [][]byte{
				createMetricRecord(),
			},
			wantResourceCount:  1,
			wantMetricCount:    1,
			wantDatapointCount: 1,
		},
		"WithMultipleRecords": {
			records: [][]byte{
				createMetricRecord(),
				createMetricRecord(),
				createMetricRecord(),
				createMetricRecord(),
				createMetricRecord(),
				createMetricRecord(),
			},
			wantResourceCount:  6,
			wantMetricCount:    6,
			wantDatapointCount: 6,
		},
		"WithEmptyRecord": {
			records:            make([][]byte, 0),
			wantResourceCount:  0,
			wantMetricCount:    0,
			wantDatapointCount: 0,
		},
		"WithInvalidRecords": {
			records:            [][]byte{{1, 2}},
			wantResourceCount:  0,
			wantMetricCount:    0,
			wantDatapointCount: 0,
		},
		"WithSomeInvalidRecords": {
			records: [][]byte{
				createMetricRecord(),
				{1, 2},
				createMetricRecord(),
			},
			wantResourceCount:  2,
			wantMetricCount:    2,
			wantDatapointCount: 2,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := unmarshaler.UnmarshalMetrics("application/x-protobuf", testCase.records)
			require.NoError(t, err)
			require.Equal(t, testCase.wantResourceCount, got.ResourceMetrics().Len())
			require.Equal(t, testCase.wantMetricCount, got.MetricCount())
			require.Equal(t, testCase.wantDatapointCount, got.DataPointCount())
		})
	}
}
