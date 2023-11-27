// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package resourcetotelemetry

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestConvertResourceToAttributes(t *testing.T) {
	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	// Before converting resource to labels
	assert.Equal(t, 1, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Len())

	md = convertToMetricsAttributes(md)

	// After converting resource to labels
	assert.Equal(t, 1, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 2, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Len())
}

func TestConvertResourceToAttributesAllDataTypesEmptyDataPoint(t *testing.T) {
	md := testdata.GenerateMetricsAllTypesEmptyDataPoint()
	assert.NotNil(t, md)

	// Before converting resource to labels
	assert.Equal(t, 1, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Sum().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Sum().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Histogram().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(5).Summary().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(6).ExponentialHistogram().DataPoints().At(0).Attributes().Len())

	md = convertToMetricsAttributes(md)

	// After converting resource to labels
	assert.Equal(t, 1, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Sum().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Sum().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Histogram().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(5).Summary().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(6).ExponentialHistogram().DataPoints().At(0).Attributes().Len())

}

func BenchmarkJoinAttributes(b *testing.B) {
	type args struct {
		from int
		to   int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "merge 10 into 10",
			args: args{
				from: 10,
				to:   10,
			},
		},
		{
			name: "merge 10 into 20",
			args: args{
				from: 10,
				to:   20,
			},
		},
		{
			name: "merge 20 into 10",
			args: args{
				from: 20,
				to:   10,
			},
		},
		{
			name: "merge 30 into 10",
			args: args{
				from: 30,
				to:   10,
			},
		},
		{
			name: "merge 10 into 30",
			args: args{
				from: 10,
				to:   30,
			},
		},
	}
	b.ReportAllocs()
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			from := initMetricAttributes(tt.args.from, 0)
			for i := 0; i < b.N; i++ {
				to := initMetricAttributes(tt.args.to, tt.args.from)
				joinAttributeMaps(from, to)
			}
		})
	}

}

func initMetricAttributes(capacity int, idx int) pcommon.Map {
	dest := pcommon.NewMap()
	dest.EnsureCapacity(capacity)
	for i := 0; i < capacity; i++ {
		dest.PutStr(fmt.Sprintf("label-name-for-index-%d", i+idx), fmt.Sprintf("label-value-for-index-%d", i+idx))
	}
	return dest
}
