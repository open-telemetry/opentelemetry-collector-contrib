// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/prometheusapiserverextension/prometheusapiservertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

var metricSet2 = `# HELP http_connected connected clients
# TYPE http_connected counter
http_connected_total{method="post",port="6380"} 15
http_connected_total{method="get",port="6380"} 12
# HELP foo_gauge_total foo gauge with _total suffix
# TYPE foo_gauge_total gauge
foo_gauge_total{method="post",port="6380"} 7
foo_gauge_total{method="get",port="6380"} 13
# EOF
`

// TestReportExtraScrapeMetrics validates 3 extra scrape metrics are reported when flag is set to true.
func TestPrometheusAPIServerEnabled(t *testing.T) {
	targets := []*testData{
		{
			name: "target",
			pages: []mockPrometheusResponse{
				{code: 200, data: metricSet2, useOpenMetrics: true},
			},
			normalizedName: false,
		},
	}

	testPrometheusAPIServer(t, targets, true)
}

func newExtensionHost() component.Host {
	return prometheusapiservertest.NewPrometheusAPIServerHost().WithPrometheusServerAPIExtension("prometheus_api_server_extension")
}

// starts prometheus receiver with custom config, retrieves metrics from MetricsSink
func testPrometheusAPIServer(t *testing.T, targets []*testData, enabled bool) {
	ctx := context.Background()
	mp, cfg, err := setupMockPrometheus(targets...)
	require.Nilf(t, err, "Failed to create Prometheus config: %v", err)
	defer mp.Close()

	cms := new(consumertest.MetricsSink)
	receiver := newPrometheusReceiver(receivertest.NewNopCreateSettings(), &Config{
		PrometheusConfig:         cfg,
		UseStartTimeMetric:       false,
		StartTimeMetricRegex:     "",
		PrometheusAPIServerExtension: &PrometheusAPIServerExtension{
			Enabled: true,
			ExtensionName: "prometheus_api_server_extension",
		},
	}, cms)

	assert.Equal(t, true, receiver.cfg.PrometheusAPIServerExtension.Enabled)
	require.NoError(t, receiver.Start(ctx, newExtensionHost()))
	assert.NotNil(t, receiver.apiExtension)
}
