// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwmetricstream

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
)

const (
	testRegion     = "us-east-1"
	testAccountID  = "1234567890"
	testStreamName = "MyMetricStream"
	testInstanceID = "i-1234567890abcdef0"
)

func TestType(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
	require.Equal(t, TypeStr, unmarshaler.Type())
}

func TestUnmarshal(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
	testCases := map[string]struct {
		filename           string
		wantResourceCount  int
		wantMetricCount    int
		wantDatapointCount int
		wantErr            error
	}{
		"WithMultipleRecords": {
			filename:           "multiple_records",
			wantResourceCount:  6,
			wantMetricCount:    33,
			wantDatapointCount: 127,
		},
		"WithSingleRecord": {
			filename:           "single_record",
			wantResourceCount:  1,
			wantMetricCount:    1,
			wantDatapointCount: 1,
		},
		"WithInvalidRecords": {
			filename: "invalid_records",
			wantErr:  errInvalidRecords,
		},
		"WithSomeInvalidRecords": {
			filename:           "some_invalid_records",
			wantResourceCount:  5,
			wantMetricCount:    36,
			wantDatapointCount: 88,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join("testdata", testCase.filename))
			require.NoError(t, err)

			got, err := unmarshaler.UnmarshalMetrics(record)
			if testCase.wantErr != nil {
				require.Error(t, err)
				require.Equal(t, testCase.wantErr, err)
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

func TestUnmarshal_SingleRecord(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())

	record, err := os.ReadFile(filepath.Join("testdata", "single_record"))
	require.NoError(t, err)
	metrics, err := unmarshaler.UnmarshalMetrics(record)
	require.NoError(t, err)

	rms := metrics.ResourceMetrics()
	require.Equal(t, 1, rms.Len())
	rm := rms.At(0)

	// Check one resource attribute to check things are wired up.
	// Remaining resource attributes are checked in TestSetResourceAttributes.
	res := rm.Resource()
	cloudProvider, ok := res.Attributes().Get(string(conventions.CloudProviderKey))
	require.True(t, ok)
	assert.Equal(t, conventions.CloudProviderAWS.Value.AsString(), cloudProvider.Str())
	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, metadata.ScopeName, sm.Scope().Name())
	assert.Equal(t, component.NewDefaultBuildInfo().Version, sm.Scope().Version())

	require.Equal(t, 1, sm.Metrics().Len())
	metric := sm.Metrics().At(0)
	assert.Equal(t, "DiskWriteOps", metric.Name())
	assert.Equal(t, "Seconds", metric.Unit())
	require.Equal(t, pmetric.MetricTypeSummary, metric.Type())
	summary := metric.Summary()
	require.Equal(t, 1, summary.DataPoints().Len())
	dp := summary.DataPoints().At(0)
	assert.Equal(t, map[string]any{"service.instance.id": "i-123456789012"}, dp.Attributes().AsRaw())
	assert.Equal(t, pcommon.Timestamp(1611929698000000000), dp.Timestamp())
	assert.Equal(t, uint64(3), dp.Count())
	assert.Equal(t, 20.0, dp.Sum())
	require.Equal(t, 4, dp.QuantileValues().Len())
	dp.QuantileValues().Sort(func(a, b pmetric.SummaryDataPointValueAtQuantile) bool {
		return a.Quantile() < b.Quantile()
	})
	q0 := dp.QuantileValues().At(0)
	q1 := dp.QuantileValues().At(1)
	q2 := dp.QuantileValues().At(2)
	q3 := dp.QuantileValues().At(3)
	assert.Equal(t, 0.0, q0.Quantile()) // min
	assert.Equal(t, 0.0, q0.Value())
	assert.Equal(t, 0.9, q1.Quantile()) // p90
	assert.Equal(t, 16.0, q1.Value())
	assert.Equal(t, 0.99, q2.Quantile()) // p99
	assert.Equal(t, 17.0, q2.Value())
	assert.Equal(t, 1.0, q3.Quantile()) // max
	assert.Equal(t, 18.0, q3.Value())
}

func TestSetDataPointAttributes(t *testing.T) {
	metric := cWMetric{
		Dimensions: map[string]string{
			"InstanceId":      testInstanceID,
			"CustomDimension": "whatever",
		},
	}
	want := map[string]any{
		string(conventions.ServiceInstanceIDKey): testInstanceID,
		"CustomDimension":                        "whatever",
	}

	dp := pmetric.NewSummaryDataPoint()
	setDataPointAttributes(metric, dp)
	require.Equal(t, want, dp.Attributes().AsRaw())
}

func TestSetResourceAttributes(t *testing.T) {
	testCases := map[string]struct {
		namespace string
		want      map[string]any
	}{
		"WithAWSNamespace": {
			namespace: "AWS/EC2",
			want: map[string]any{
				attributeAWSCloudWatchMetricStreamName:  testStreamName,
				string(conventions.CloudAccountIDKey):   testAccountID,
				string(conventions.CloudRegionKey):      testRegion,
				string(conventions.CloudProviderKey):    conventions.CloudProviderAWS.Value.AsString(),
				string(conventions.ServiceNameKey):      "EC2",
				string(conventions.ServiceNamespaceKey): "AWS",
			},
		},
		"WithCustomNamespace": {
			namespace: "CustomNamespace",
			want: map[string]any{
				attributeAWSCloudWatchMetricStreamName: testStreamName,
				string(conventions.CloudAccountIDKey):  testAccountID,
				string(conventions.CloudRegionKey):     testRegion,
				string(conventions.CloudProviderKey):   conventions.CloudProviderAWS.Value.AsString(),
				string(conventions.ServiceNameKey):     "CustomNamespace",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			key := resourceKey{
				accountID:        testAccountID,
				region:           testRegion,
				metricStreamName: testStreamName,
				namespace:        testCase.namespace,
			}

			resource := pcommon.NewResource()
			setResourceAttributes(key, resource)
			require.Equal(t, testCase.want, resource.Attributes().AsRaw())
		})
	}
}
