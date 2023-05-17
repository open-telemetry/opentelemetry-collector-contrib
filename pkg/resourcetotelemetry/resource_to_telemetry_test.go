// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package resourcetotelemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestConvertResourceToAttributes(t *testing.T) {
	md := testdata.GenerateMetricsOneMetric()
	assert.NotNil(t, md)

	// Before converting resource to labels
	assert.Equal(t, 1, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Len())

	cloneMd := convertToMetricsAttributes(md)

	// After converting resource to labels
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 2, cloneMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Len())

	assert.Equal(t, 1, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Len())

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

	cloneMd := convertToMetricsAttributes(md)

	// After converting resource to labels
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Sum().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Sum().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Histogram().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(5).Summary().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 1, cloneMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(6).ExponentialHistogram().DataPoints().At(0).Attributes().Len())

	assert.Equal(t, 1, md.ResourceMetrics().At(0).Resource().Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Sum().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Sum().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(4).Histogram().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(5).Summary().DataPoints().At(0).Attributes().Len())
	assert.Equal(t, 0, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(6).ExponentialHistogram().DataPoints().At(0).Attributes().Len())

}
