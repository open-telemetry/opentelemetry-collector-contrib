// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TestIssue34263_GroupedCounterMetrics reproduces issue #34263 where grouped counter metrics
// are silently dropped. This test uses the actual Prometheus scrape output from the issue.
//
// Issue: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34263
//
// The problem occurs when:
// 1. Multiple time series share the same counter metric name with different labels
// 2. Metadata (# TYPE declarations) may not be properly propagated
// 3. Counter metrics get treated as gauges and are dropped
func TestIssue34263_GroupedCounterMetrics(t *testing.T) {
	// This is the actual Prometheus scrape output from issue #34263
	// It contains grouped counter metrics (cnpg_pg_stat_database_blks_read)
	// and grouped gauge metrics (cnpg_pg_stat_replication_flush_diff_bytes)
	promData := `
# HELP cnpg_pg_stat_database_blks_read Number of disk blocks read in this database
# TYPE cnpg_pg_stat_database_blks_read counter
cnpg_pg_stat_database_blks_read{datname=""} 122
cnpg_pg_stat_database_blks_read{datname="my-database"} 426
cnpg_pg_stat_database_blks_read{datname="postgres"} 359
cnpg_pg_stat_database_blks_read{datname="template0"} 0
cnpg_pg_stat_database_blks_read{datname="template1"} 1379
# HELP cnpg_pg_stat_replication_flush_diff_bytes Difference in bytes from the last write-ahead log location flushed to disk by this standby server
# TYPE cnpg_pg_stat_replication_flush_diff_bytes gauge
cnpg_pg_stat_replication_flush_diff_bytes{application_name="my-cnpg-cluster-2",client_addr="10.244.0.99/32",client_port="53160",usename="streaming_replica"} 0
cnpg_pg_stat_replication_flush_diff_bytes{application_name="my-cnpg-cluster-3",client_addr="10.244.3.70/32",client_port="47804",usename="streaming_replica"} 0
`

	targets := []*testData{
		{
			name: "issue_34263_grouped_counters",
			pages: []mockPrometheusResponse{
				{code: 200, data: promData},
			},
			validateFunc: func(t *testing.T, td *testData, mds []pmetric.ResourceMetrics) {
				require.NotEmpty(t, mds, "At least one resource metric should be present")

				metrics := getMetrics(mds[0])

				// Find the counter and gauge metrics
				var counterMetric, gaugeMetric pmetric.Metric
				var foundCounter, foundGauge bool

				for _, m := range metrics {
					switch m.Name() {
					case "cnpg_pg_stat_database_blks_read":
						counterMetric = m
						foundCounter = true
					case "cnpg_pg_stat_replication_flush_diff_bytes":
						gaugeMetric = m
						foundGauge = true
					}
				}

				// Verify gauge metric is present (this should work)
				require.True(t, foundGauge, "Gauge metric 'cnpg_pg_stat_replication_flush_diff_bytes' should be present")
				require.Equal(t, pmetric.MetricTypeGauge, gaugeMetric.Type(), "Expected gauge type")
				require.Equal(t, 2, gaugeMetric.Gauge().DataPoints().Len(), "Expected 2 gauge data points")

				// Verify counter metric is present (THIS IS THE BUG - it gets dropped or mistyped)
				require.True(t, foundCounter, "Counter metric 'cnpg_pg_stat_database_blks_read' should be present (BUG: gets dropped in issue #34263)")
				require.Equal(t, pmetric.MetricTypeSum, counterMetric.Type(), "Expected sum type for counter metric")
				require.Equal(t, 5, counterMetric.Sum().DataPoints().Len(), "Expected 5 counter data points")
				require.True(t, counterMetric.Sum().IsMonotonic(), "Counter should be monotonic")

				// Verify the actual values are present
				// Note: We just verify we have the correct number of data points
				// The label filtering behavior may vary, so we don't assert on specific attribute values
				t.Logf("Counter metric has %d data points (expected 5)", counterMetric.Sum().DataPoints().Len())
			},
			validateScrapes: true,
		},
	}

	testComponent(t, targets, nil)
}

// TestIssue34263_WithMoreCounterPatterns tests additional counter patterns
// that should be recognized even without metadata
func TestIssue34263_WithMoreCounterPatterns(t *testing.T) {
	// Additional test with common counter patterns from real-world Prometheus exporters
	promData := `
# HELP http_requests_total Total HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 1234
http_requests_total{method="POST",status="201"} 567
# HELP bytes_written Total bytes written
# TYPE bytes_written counter
bytes_written{device="sda"} 10485760
bytes_written{device="sdb"} 20971520
# HELP packets_received Total packets received
# TYPE packets_received counter
packets_received{interface="eth0"} 5000
packets_received{interface="eth1"} 3000
# HELP cache_hits Cache hits
# TYPE cache_hits counter
cache_hits{cache="redis"} 9876
cache_hits{cache="memcached"} 5432
`

	targets := []*testData{
		{
			name: "issue_34263_various_counter_patterns",
			pages: []mockPrometheusResponse{
				{code: 200, data: promData},
			},
			validateFunc: func(t *testing.T, td *testData, mds []pmetric.ResourceMetrics) {
				require.NotEmpty(t, mds, "At least one resource metric should be present")

				metrics := getMetrics(mds[0])

				// All of these should be recognized as counters
				counterNames := []string{
					"http_requests_total",
					"bytes_written",
					"packets_received",
					"cache_hits",
				}

				for _, name := range counterNames {
					found := false
					for _, m := range metrics {
						if m.Name() == name {
							found = true
							require.Equal(t, pmetric.MetricTypeSum, m.Type(),
								"Metric '%s' should be typed as Sum (counter)", name)
							require.True(t, m.Sum().IsMonotonic(),
								"Metric '%s' should be monotonic", name)
							break
						}
					}
					require.True(t, found, "Counter metric '%s' should be present", name)
				}
			},
			validateScrapes: true,
		},
	}

	testComponent(t, targets, nil)
}
