// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpmetricstream

import (
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"
)

func TestType(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop())
	require.Equal(t, TypeStr, unmarshaler.Type())
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
			got, err := unmarshaler.Unmarshal(testCase.records)
			require.NoError(t, err)
			require.Equal(t, testCase.wantResourceCount, got.ResourceMetrics().Len())
			require.Equal(t, testCase.wantMetricCount, got.MetricCount())
			require.Equal(t, testCase.wantDatapointCount, got.DataPointCount())
		})
	}
}
