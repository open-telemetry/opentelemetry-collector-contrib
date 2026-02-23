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

	wme := &wrapperMetricsExporter{excludeServiceAttributes: false}
	md = wme.convertToMetricsAttributes(md)

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

	wme := &wrapperMetricsExporter{excludeServiceAttributes: false}
	md = wme.convertToMetricsAttributes(md)

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

func TestConvertResourceToAttributesWithExcludeServiceAttributes(t *testing.T) {
	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	// Add service.name and service.instance.id to resource attributes
	resource := md.ResourceMetrics().At(0).Resource()
	resource.Attributes().PutStr("service.name", "test-service")
	resource.Attributes().PutStr("service.instance.id", "test-instance-id")
	resource.Attributes().PutStr("service.namespace", "test-namespace")

	// Before converting: 3 resource attrs (original + 2 service attrs), 1 datapoint attr
	assert.Equal(t, 4, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Len())

	wme := &wrapperMetricsExporter{excludeServiceAttributes: true}
	md = wme.convertToMetricsAttributes(md)

	// After converting: service.name, service.instance.id and service.namespace should NOT be added to datapoint attrs
	// Original resource attrs remain unchanged
	assert.Equal(t, 4, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	// Datapoint should have: 1 original + 1 (resource-name from testdata) = 2
	dpAttrs := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes()
	assert.Equal(t, 2, dpAttrs.Len())
	_, hasServiceName := dpAttrs.Get("service.name")
	_, hasServiceInstanceID := dpAttrs.Get("service.instance.id")
	_, hasServiceNamespace := dpAttrs.Get("service.namespace")
	assert.False(t, hasServiceName)
	assert.False(t, hasServiceInstanceID)
	assert.False(t, hasServiceNamespace)
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
			for b.Loop() {
				to := initMetricAttributes(tt.args.to, tt.args.from)
				joinAttributeMaps(from, to)
			}
		})
	}
}

func initMetricAttributes(capacity, idx int) pcommon.Map {
	dest := pcommon.NewMap()
	dest.EnsureCapacity(capacity)
	for i := range capacity {
		dest.PutStr(fmt.Sprintf("label-name-for-index-%d", i+idx), fmt.Sprintf("label-value-for-index-%d", i+idx))
	}
	return dest
}
