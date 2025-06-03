// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

func TestEnableCreatedTimestampZeroIngestionGateUsage(t *testing.T) {
	ctx := context.Background()
	mockConsumer := new(consumertest.MetricsSink)
	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)

	// Test with feature gate enabled
	err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", true)
	require.NoError(t, err)
	r1, err := newPrometheusReceiver(settings, cfg, mockConsumer)
	require.NoError(t, err)

	assert.True(t, enableCreatedTimestampZeroIngestionGate.IsEnabled(), "Feature gate should be enabled")
	opts := r1.initScrapeOptions()
	assert.True(t, opts.EnableCreatedTimestampZeroIngestion, "EnableCreatedTimestampZeroIngestion should be true when feature gate is enabled")

	// Test with feature gate disabled
	err = featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", false)
	require.NoError(t, err)
	r2, err := newPrometheusReceiver(settings, cfg, mockConsumer)
	require.NoError(t, err)

	assert.False(t, enableCreatedTimestampZeroIngestionGate.IsEnabled(), "Feature gate should be disabled")
	opts = r2.initScrapeOptions()
	assert.False(t, opts.EnableCreatedTimestampZeroIngestion, "EnableCreatedTimestampZeroIngestion should be false when feature gate is disabled")

	// Reset the feature gate and shutdown the created receivers
	t.Cleanup(func() {
		err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", false)
		require.NoError(t, err, "Failed to reset feature gate to default state")

		require.NoError(t, r1.Shutdown(ctx), "Failed to shutdown receiver 1")
		require.NoError(t, r2.Shutdown(ctx), "Failed to shutdown receiver 2")
	})
}

var openMetricsCreatedTimestampMetrics = `# HELP a_seconds A counter
# TYPE a_seconds counter
# UNIT a_seconds seconds
a_seconds_total 1.0
a_seconds_created 123.456
# EOF
`

func TestOpenMetricsCreatedTimestampZeroIngestionEnabled(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", true)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", false)
		require.NoError(t, err, "Failed to reset feature gate to default state")
	})

	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: openMetricsCreatedTimestampMetrics, useOpenMetrics: true},
			},
			validateFunc:    verifyOpenMetricsCreatedTimestampZeroIngestionEnabled,
			validateScrapes: true,
			normalizedName:  true,
		},
	}

	testComponent(t, targets, nil)
}

func verifyOpenMetricsCreatedTimestampZeroIngestionEnabled(t *testing.T, td *testData, mds []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, mds)
	ts1 := getTS(mds[0].ScopeMetrics().At(0).Metrics())
	e1 := []metricExpectation{
		{
			"a_seconds_total",
			pmetric.MetricTypeSum,
			"s",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(timestampFromFloat64(123.456)),
						compareTimestamp(ts1),
						compareDoubleValue(1.0),
					},
				},
			},
			nil,
		},
	}
	doCompare(t, "created-timestamp-zero-ingestion-enabled", td.attributes, mds[0], e1)
}

func TestOpenMetricsCreatedTimestampZeroIngestionDisabled(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", false)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := featuregate.GlobalRegistry().Set("receiver.prometheusreceiver.EnableCreatedTimestampZeroIngestion", false)
		require.NoError(t, err, "Failed to reset feature gate to default state")
	})

	targets := []*testData{
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: openMetricsCreatedTimestampMetrics, useOpenMetrics: true},
			},
			validateFunc:    verifyOpenMetricsCreatedTimestampZeroIngestionDisabled,
			validateScrapes: true,
			normalizedName:  false,
		},
	}

	testComponent(t, targets, nil)
}

func verifyOpenMetricsCreatedTimestampZeroIngestionDisabled(t *testing.T, td *testData, mds []pmetric.ResourceMetrics) {
	verifyNumValidScrapeResults(t, td, mds)
	ts1 := getTS(mds[0].ScopeMetrics().At(0).Metrics())
	e1 := []metricExpectation{
		{
			"a_seconds_total",
			pmetric.MetricTypeSum,
			"s",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(ts1),
						compareTimestamp(ts1),
						compareDoubleValue(1.0),
					},
				},
			},
			nil,
		},
		{
			"a_seconds_created",
			pmetric.MetricTypeSum,
			"s",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareStartTimestamp(timestampFromFloat64(123.456)),
						compareTimestamp(ts1),
						compareDoubleValue(0),
					},
				},
			},
			nil,
		},
	}

	doCompare(t, "created-timestamp-zero-ingestion-disabled", td.attributes, mds[0], e1)
}

func timestampFromFloat64(ts float64) pcommon.Timestamp {
	secs := int64(ts)
	nanos := int64((ts - float64(secs)) * 1e9)
	return pcommon.Timestamp(secs*1e9 + nanos)
}
