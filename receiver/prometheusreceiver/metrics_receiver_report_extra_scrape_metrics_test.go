// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

var metricSet = `# HELP http_connected connected clients
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
func TestReportExtraScrapeMetrics(t *testing.T) {
	target := func(reportExtraScrapeMetrics bool) *testData {
		return &testData{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: metricSet, useOpenMetrics: true},
			},
			normalizedName: false,
			validateFunc: func(t *testing.T, td *testData, result []pmetric.ResourceMetrics) {
				verifyMetrics(t, td, result, reportExtraScrapeMetrics)
			},
		}
	}

	testScraperMetrics(t, []*testData{target(false)}, false) // extraScrapeMetrics flag is false
	testScraperMetrics(t, []*testData{target(true)}, true)   // extraScrapeMetrics flag is true
}

// starts prometheus receiver with custom config, retrieves metrics from MetricsSink
func testScraperMetrics(t *testing.T, targets []*testData, reportExtraScrapeMetrics bool) {
	ctx := context.Background()
	mp, cfg, err := setupMockPrometheus(targets...)
	require.NoErrorf(t, err, "Failed to create Prometheus config: %v", err)
	defer mp.Close()

	cms := new(consumertest.MetricsSink)
	receiver, err := newPrometheusReceiver(receivertest.NewNopSettings(metadata.Type), &Config{
		PrometheusConfig:         cfg,
		UseStartTimeMetric:       false,
		StartTimeMetricRegex:     "",
		ReportExtraScrapeMetrics: reportExtraScrapeMetrics,
	}, cms)
	require.NoError(t, err, "Failed to create Prometheus receiver: %v", err)

	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
	// verify state after shutdown is called
	t.Cleanup(func() {
		// verify state after shutdown is called
		assert.Lenf(t, flattenTargets(receiver.scrapeManager.TargetsAll()), len(targets), "expected %v targets to be running", len(targets))
		require.NoError(t, receiver.Shutdown(context.Background()))
		assert.Empty(t, flattenTargets(receiver.scrapeManager.TargetsAll()), "expected scrape manager to have no targets")
	})

	// waitgroup Wait() is strictly from a server POV indicating the sufficient number and type of requests have been seen
	mp.wg.Wait()

	// Note:waitForScrapeResult is an attempt to address a possible race between waitgroup Done() being called in the ServerHTTP function
	//      and when the receiver actually processes the http request responses into metrics.
	//      this is a eventually timeout,tick that just waits for some condition.
	//      however the condition to wait for may be suboptimal and may need to be adjusted.
	waitForScrapeResults(t, targets, cms)

	// This begins the processing of the scrapes collected by the receiver
	metrics := cms.AllMetrics()
	// split and store results by target name
	pResults := splitMetricsByTarget(metrics)
	lres, lep := len(pResults), len(mp.endpoints)
	// There may be an additional scrape entry between when the mock server provided
	// all responses and when we capture the metrics.  It will be ignored later.
	assert.GreaterOrEqualf(t, lep, lres, "want at least %d targets, but got %v\n", lep, lres)

	// loop to validate outputs for each targets
	// Stop once we have evaluated all expected results, any others are superfluous.
	for _, target := range targets[:lep] {
		t.Run(target.name, func(t *testing.T) {
			name := target.name
			scrapes := pResults[name]
			if !target.validateScrapes {
				scrapes = getValidScrapes(t, pResults[name], target)
				assert.GreaterOrEqual(t, 1, len(scrapes))
				if reportExtraScrapeMetrics {
					// scrapes has 2 prom metrics + 5 internal scraper metrics + 3 internal extra scraper metrics = 10
					// scrape_sample_limit, scrape_timeout_seconds, scrape_body_size_bytes
					assert.Equal(t, 2+expectedExtraScrapeMetricCount, metricsCount(scrapes[0]))
				} else {
					// scrapes has 2 prom metrics + 5 internal scraper metrics = 7
					assert.Equal(t, 2+expectedScrapeMetricCount, metricsCount(scrapes[0]))
				}
			}
			target.validateFunc(t, target, scrapes)
		})
	}
}

func verifyMetrics(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics, reportExtraScrapeMetrics bool) {
	verifyNumValidScrapeResults(t, td, resourceMetrics)
	m1 := resourceMetrics[0]

	wantAttributes := td.attributes

	metrics1 := m1.ScopeMetrics().At(0).Metrics()
	ts1 := getTS(metrics1)
	e1 := []metricExpectation{
		{
			"http_connected_total",
			pmetric.MetricTypeSum,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(15),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(12),
						compareAttributes(map[string]string{"method": "get", "port": "6380"}),
					},
				},
			},
			nil,
		},
		{
			"foo_gauge_total",
			pmetric.MetricTypeGauge,
			"",
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(7),
						compareAttributes(map[string]string{"method": "post", "port": "6380"}),
					},
				},
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(13),
						compareAttributes(map[string]string{"method": "get", "port": "6380"}),
					},
				},
			},
			nil,
		},
	}

	if reportExtraScrapeMetrics {
		e1 = append(e1, []metricExpectation{
			{
				"scrape_body_size_bytes",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
			{
				"scrape_sample_limit",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
			{
				"scrape_timeout_seconds",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
		}...)
	}

	doCompare(t, "scrape-reportExtraScrapeMetrics-1", wantAttributes, m1, e1)
}
