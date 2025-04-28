// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpmetricstream

import (
	"slices"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
)

func TestType(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
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
	minQ := qv.AppendEmpty()
	minQ.SetQuantile(0)
	minQ.SetValue(0)
	maxQ := qv.AppendEmpty()
	maxQ.SetQuantile(1)
	maxQ.SetValue(1)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	temp, _ := er.MarshalProto()
	record := proto.EncodeVarint(uint64(len(temp)))
	record = append(record, temp...)
	return record
}

func TestUnmarshal(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
	testCases := map[string]struct {
		record             []byte
		wantResourceCount  int
		wantMetricCount    int
		wantDatapointCount int
		wantErr            string
	}{
		"WithSingleRecord": {
			record:             createMetricRecord(),
			wantResourceCount:  1,
			wantMetricCount:    1,
			wantDatapointCount: 1,
		},
		"WithMultipleRecords": {
			record: slices.Concat(
				createMetricRecord(),
				createMetricRecord(),
				createMetricRecord(),
				createMetricRecord(),
				createMetricRecord(),
				createMetricRecord(),
			),
			wantResourceCount:  6,
			wantMetricCount:    6,
			wantDatapointCount: 6,
		},
		"WithEmptyRecord": {
			record:             []byte{},
			wantResourceCount:  0,
			wantMetricCount:    0,
			wantDatapointCount: 0,
		},
		"WithInvalidRecord": {
			record:             []byte{1, 2},
			wantResourceCount:  0,
			wantMetricCount:    0,
			wantDatapointCount: 0,
			wantErr:            "unable to unmarshal input: proto: ExportMetricsServiceRequest: illegal tag 0 (wire type 2)",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := unmarshaler.UnmarshalMetrics(testCase.record)
			if testCase.wantErr != "" {
				require.Error(t, err)
				require.EqualError(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, testCase.wantResourceCount, got.ResourceMetrics().Len())
				gotMetricCount := 0
				gotDatapointCount := 0
				for i := 0; i < got.ResourceMetrics().Len(); i++ {
					rm := got.ResourceMetrics().At(i)
					require.Equal(t, 1, rm.ScopeMetrics().Len())
					ilm := rm.ScopeMetrics().At(0)
					require.Equal(t, metadata.ScopeName, ilm.Scope().Name())
					require.Equal(t, component.NewDefaultBuildInfo().Version, ilm.Scope().Version())
					gotMetricCount += ilm.Metrics().Len()
					for j := 0; j < ilm.Metrics().Len(); j++ {
						metric := ilm.Metrics().At(j)
						gotDatapointCount += metric.Summary().DataPoints().Len()
					}
				}
				require.Equal(t, testCase.wantMetricCount, gotMetricCount)
				require.Equal(t, testCase.wantDatapointCount, gotDatapointCount)
			}
		})
	}
}
