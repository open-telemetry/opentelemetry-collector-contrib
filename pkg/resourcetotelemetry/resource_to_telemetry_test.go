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

func TestConvertResourceToAttributesWithResourceConstantLabels(t *testing.T) {
	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	resource := md.ResourceMetrics().At(0).Resource()
	resource.Attributes().PutStr("k8s.pod.name", "test-pod")
	resource.Attributes().PutStr("k8s.namespace.name", "test-namespace")
	resource.Attributes().PutStr("k8s.secret.name", "secret")
	resource.Attributes().PutStr("ignored.attribute", "ignored-value")

	wme := &wrapperMetricsExporter{
		constantLabelsMatcher: newResourceAttributeMatcher(Settings{
			Included: []string{"k8s.*"},
			Excluded: []string{"*.secret.*"},
		}),
	}
	md = wme.convertToMetricsAttributes(md)

	dpAttrs := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes()
	_, hasPodName := dpAttrs.Get("k8s.pod.name")
	_, hasNamespaceName := dpAttrs.Get("k8s.namespace.name")
	_, hasSecret := dpAttrs.Get("k8s.secret.name")
	_, hasIgnored := dpAttrs.Get("ignored.attribute")
	assert.True(t, hasPodName)
	assert.True(t, hasNamespaceName)
	assert.False(t, hasSecret)
	assert.False(t, hasIgnored)
}

func TestConvertResourceToAttributesWithEmptyIncludedAndNonEmptyExcluded(t *testing.T) {
	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	resource := md.ResourceMetrics().At(0).Resource()
	resource.Attributes().PutStr("k8s.pod.name", "test-pod")
	resource.Attributes().PutStr("k8s.secret.name", "secret")

	wme := &wrapperMetricsExporter{
		constantLabelsMatcher: newResourceAttributeMatcher(Settings{
			Excluded: []string{"*.secret.*"},
		}),
	}
	md = wme.convertToMetricsAttributes(md)

	dpAttrs := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes()
	_, hasPodName := dpAttrs.Get("k8s.pod.name")
	_, hasSecret := dpAttrs.Get("k8s.secret.name")
	assert.True(t, hasPodName)
	assert.False(t, hasSecret)
}

func TestSettings_Validate(t *testing.T) {
	valid := Settings{Included: []string{"foo*", "bar?"}, Excluded: []string{"*secret*"}}
	assert.NoError(t, valid.Validate())
	assert.False(t, valid.IsEmpty())

	empty := Settings{}
	assert.True(t, empty.IsEmpty())
	assert.NoError(t, empty.Validate())

	invalid := Settings{Enabled: true, Included: []string{"foo*"}}
	assert.Error(t, invalid.Validate())
}

func TestWrapMetricsExporterWithSettingsResourceConstantLabels(t *testing.T) {
	exp := &wrapperMetricsExporter{}
	set := Settings{
		Included: []string{"attr1"},
	}
	assert.NoError(t, set.Validate())

	wrapped := WrapMetricsExporter(set, exp)
	wme, ok := wrapped.(*wrapperMetricsExporter)
	assert.True(t, ok)
	assert.NotNil(t, wme.constantLabelsMatcher)
	assert.True(t, wme.constantLabelsMatcher.matches("attr1"))
	assert.False(t, wme.constantLabelsMatcher.matches("attr2"))
}

func TestWrapMetricsExporterDefault(t *testing.T) {
	exp := &wrapperMetricsExporter{}
	wrapped := WrapMetricsExporter(Settings{}, exp)
	assert.Same(t, exp, wrapped)
}

func TestConvertResourceToAttributesWithWildcardAllAndNewline(t *testing.T) {
	md := testdata.GenerateMetricsOneMetric()
	resource := md.ResourceMetrics().At(0).Resource()
	resource.Attributes().PutStr("key.with\nnewline", "val1")
	resource.Attributes().PutStr("regular.key", "val2")

	wme := &wrapperMetricsExporter{
		constantLabelsMatcher: newResourceAttributeMatcher(Settings{
			Included: []string{"*"},
		}),
	}
	md = wme.convertToMetricsAttributes(md)

	dpAttrs := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes()
	val1, hasVal1 := dpAttrs.Get("key.with\nnewline")
	assert.True(t, hasVal1)
	assert.Equal(t, "val1", val1.Str())

	val2, hasVal2 := dpAttrs.Get("regular.key")
	assert.True(t, hasVal2)
	assert.Equal(t, "val2", val2.Str())
}

func TestConvertResourceToAttributesWithExactMatches(t *testing.T) {
	md := testdata.GenerateMetricsOneMetric()
	resource := md.ResourceMetrics().At(0).Resource()
	resource.Attributes().PutStr("exact.match", "val1")
	resource.Attributes().PutStr("excluded.exact", "val2")
	resource.Attributes().PutStr("other.key", "val3")

	wme := &wrapperMetricsExporter{
		constantLabelsMatcher: newResourceAttributeMatcher(Settings{
			Included: []string{"exact.match", "other.pattern*"},
			Excluded: []string{"excluded.exact"},
		}),
	}
	md = wme.convertToMetricsAttributes(md)

	dpAttrs := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes()
	_, hasExact := dpAttrs.Get("exact.match")
	_, hasExcluded := dpAttrs.Get("excluded.exact")
	_, hasOther := dpAttrs.Get("other.key")
	assert.True(t, hasExact)
	assert.False(t, hasExcluded)
	assert.False(t, hasOther)
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
