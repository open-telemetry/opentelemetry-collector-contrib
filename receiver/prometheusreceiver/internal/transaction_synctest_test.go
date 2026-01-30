// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build goexperiment.synctest

package internal

import (
	"context"
	"io"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// scrapeResult represents the metrics collected from a single scrape.
type scrapeResult struct {
	metrics     pmetric.Metrics
	scrapeError error
}

// testScraper provides a simplified interface for unit testing the transaction layer.
// It directly invokes the transaction's Append/Commit cycle without HTTP.
type testScraper struct {
	t           *testing.T
	ctx         context.Context
	sink        *consumertest.MetricsSink
	jobName     string
	instance    string
	metadata    map[string]scrape.MetricMetadata
	trimSuffix  bool
	useMetadata bool
}

// newTestScraper creates a new test scraper for unit testing.
func newTestScraper(t *testing.T, jobName, instance string) *testScraper {
	t.Helper()

	// Create target labels
	targetLabels := labels.FromMap(map[string]string{
		model.InstanceLabel: instance,
		model.JobLabel:      jobName,
	})

	target := scrape.NewTarget(
		targetLabels,
		&promconfig.ScrapeConfig{},
		map[model.LabelName]model.LabelValue{
			model.AddressLabel: model.LabelValue(instance),
			model.SchemeLabel:  "http",
		},
		nil,
	)

	// Build metadata from the well-known test metrics.
	metadata := buildDefaultMetadata()

	// Use context.Background() instead of t.Context() because synctest requires
	// all context operations to happen within the synctest bubble.
	ctx := scrape.ContextWithTarget(context.Background(), target)
	ctx = scrape.ContextWithMetricMetadataStore(ctx, testMetadataStore(metadata))

	return &testScraper{
		t:           t,
		ctx:         ctx,
		sink:        new(consumertest.MetricsSink),
		jobName:     jobName,
		instance:    instance,
		metadata:    metadata,
		trimSuffix:  false,
		useMetadata: true,
	}
}

// buildDefaultMetadata returns metadata for all metrics used in the core e2e tests.
func buildDefaultMetadata() map[string]scrape.MetricMetadata {
	return map[string]scrape.MetricMetadata{
		// target1-3 metrics
		"go_threads":                    {MetricFamily: "go_threads", Type: model.MetricTypeGauge},
		"http_requests_total":           {MetricFamily: "http_requests_total", Type: model.MetricTypeCounter},
		"http_request_duration_seconds": {MetricFamily: "http_request_duration_seconds", Type: model.MetricTypeHistogram},
		"rpc_duration_seconds":          {MetricFamily: "rpc_duration_seconds", Type: model.MetricTypeSummary},
		"corrupted_hist":                {MetricFamily: "corrupted_hist", Type: model.MetricTypeHistogram},
		// target4 metrics
		"foo":       {MetricFamily: "foo", Type: model.MetricTypeCounter},
		"foo_total": {MetricFamily: "foo_total", Type: model.MetricTypeCounter},
		// Standard scrape metrics
		"up":                                    {MetricFamily: "up", Type: model.MetricTypeGauge},
		"scrape_duration_seconds":               {MetricFamily: "scrape_duration_seconds", Type: model.MetricTypeGauge},
		"scrape_samples_scraped":                {MetricFamily: "scrape_samples_scraped", Type: model.MetricTypeGauge},
		"scrape_samples_post_metric_relabeling": {MetricFamily: "scrape_samples_post_metric_relabeling", Type: model.MetricTypeGauge},
		"scrape_series_added":                   {MetricFamily: "scrape_series_added", Type: model.MetricTypeGauge},
	}
}

// scrape simulates a Prometheus scrape by parsing the provided text and feeding it to the transaction.
func (ts *testScraper) scrape(promText string, atMs int64) scrapeResult {
	ts.t.Helper()

	tr := newTransaction(
		ts.ctx,
		ts.sink,
		labels.EmptyLabels(),
		receivertest.NewNopSettings(receivertest.NopType),
		nopObsRecv(ts.t),
		ts.trimSuffix,
		ts.useMetadata,
	)

	// Parse the Prometheus text format and feed to transaction
	err := parseAndAppend(ts.t, tr, promText, atMs, ts.jobName, ts.instance)
	if err != nil {
		return scrapeResult{scrapeError: err}
	}

	if err := tr.Commit(); err != nil {
		return scrapeResult{scrapeError: err}
	}

	// Get the metrics that were sent to the sink
	allMetrics := ts.sink.AllMetrics()
	if len(allMetrics) == 0 {
		return scrapeResult{metrics: pmetric.NewMetrics()}
	}

	// Return the last metrics (the ones from this scrape)
	return scrapeResult{metrics: allMetrics[len(allMetrics)-1]}
}

// allMetrics returns all metrics collected so far.
func (ts *testScraper) allMetrics() []pmetric.Metrics {
	return ts.sink.AllMetrics()
}

// parseAndAppend parses Prometheus text format and appends to the transaction.
func parseAndAppend(t *testing.T, tr *transaction, promText string, atMs int64, jobName, instance string) error {
	t.Helper()

	if promText == "" {
		return nil
	}

	// Use NewPromParser directly for the Prometheus text exposition format.
	// This avoids issues with content-type detection that the generic New() function has.
	p := textparse.NewPromParser([]byte(promText), labels.NewSymbolTable(), false)

	for {
		et, err := p.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		switch et {
		case textparse.EntrySeries:
			_, _, val := p.Series()
			var lbls labels.Labels
			p.Labels(&lbls)

			// Add job and instance labels if not present
			b := labels.NewBuilder(lbls)
			if lbls.Get(model.JobLabel) == "" {
				b.Set(model.JobLabel, jobName)
			}
			if lbls.Get(model.InstanceLabel) == "" {
				b.Set(model.InstanceLabel, instance)
			}
			lbls = b.Labels()

			_, err = tr.Append(0, lbls, atMs, val)
			if err != nil {
				// Log but don't fail on individual append errors (matches scrape behavior)
				t.Logf("Append error for %s: %v", lbls.String(), err)
			}

		case textparse.EntryType, textparse.EntryHelp, textparse.EntryComment, textparse.EntryUnit:
			// Skip metadata entries - we use our own metadata store
			continue
		}
	}

	return nil
}

// Test data from metrics_receiver_test.go
const testTarget1Page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 100
http_requests_total{method="post",code="400"} 5

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 1000
http_request_duration_seconds_bucket{le="0.5"} 1500
http_request_duration_seconds_bucket{le="1"} 2000
http_request_duration_seconds_bucket{le="+Inf"} 2500
http_request_duration_seconds_sum 5000
http_request_duration_seconds_count 2500

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 5
rpc_duration_seconds{quantile="0.99"} 8
rpc_duration_seconds_sum 5000
rpc_duration_seconds_count 1000
`

const testTarget1Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 199
http_requests_total{method="post",code="400"} 12

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 1100
http_request_duration_seconds_bucket{le="0.5"} 1600
http_request_duration_seconds_bucket{le="1"} 2100
http_request_duration_seconds_bucket{le="+Inf"} 2600
http_request_duration_seconds_sum 5050
http_request_duration_seconds_count 2600

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 6
rpc_duration_seconds{quantile="0.99"} 8
rpc_duration_seconds_sum 5002
rpc_duration_seconds_count 1001
`

// TestMultipleScrapeSequenceWithSynctest tests a sequence of scrapes using synctest
// for deterministic time control.
func TestMultipleScrapeSequenceWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		// First scrape
		baseTime := time.Now()
		result1 := scraper.scrape(testTarget1Page1, baseTime.UnixMilli())
		require.NoError(t, result1.scrapeError)

		// Advance time by 15 seconds (simulating scrape interval)
		time.Sleep(15 * time.Second)

		// Second scrape
		scrape2Time := time.Now()
		result2 := scraper.scrape(testTarget1Page2, scrape2Time.UnixMilli())
		require.NoError(t, result2.scrapeError)

		// Verify we got metrics from both scrapes
		allMetrics := scraper.allMetrics()
		require.Len(t, allMetrics, 2, "expected 2 scrape results")

		// Verify first scrape metrics
		verifyFirstScrapeMetrics(t, allMetrics[0])

		// Verify second scrape metrics
		verifySecondScrapeMetrics(t, allMetrics[1])
	})
}

// TestCounterMetricsWithSynctest tests counter metric processing.
func TestCounterMetricsWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		promText := `
# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 100
http_requests_total{method="post",code="400"} 5
`
		baseTime := time.Now()
		result := scraper.scrape(promText, baseTime.UnixMilli())
		require.NoError(t, result.scrapeError)

		// Verify counter metrics
		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		found := false
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "http_requests_total" {
				found = true
				assert.Equal(t, pmetric.MetricTypeSum, m.Type())
				assert.True(t, m.Sum().IsMonotonic())
				assert.Equal(t, pmetric.AggregationTemporalityCumulative, m.Sum().AggregationTemporality())
				assert.Equal(t, 2, m.Sum().DataPoints().Len())
			}
		}
		assert.True(t, found, "http_requests_total metric not found")
	})
}

// TestGaugeMetricsWithSynctest tests gauge metric processing.
func TestGaugeMetricsWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		promText := `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19
`
		baseTime := time.Now()
		result := scraper.scrape(promText, baseTime.UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		found := false
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "go_threads" {
				found = true
				assert.Equal(t, pmetric.MetricTypeGauge, m.Type())
				assert.Equal(t, 1, m.Gauge().DataPoints().Len())
				assert.Equal(t, float64(19), m.Gauge().DataPoints().At(0).DoubleValue())
			}
		}
		assert.True(t, found, "go_threads metric not found")
	})
}

// TestHistogramMetricsWithSynctest tests histogram metric processing.
func TestHistogramMetricsWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		promText := `
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 1000
http_request_duration_seconds_bucket{le="0.5"} 1500
http_request_duration_seconds_bucket{le="1"} 2000
http_request_duration_seconds_bucket{le="+Inf"} 2500
http_request_duration_seconds_sum 5000
http_request_duration_seconds_count 2500
`
		baseTime := time.Now()
		result := scraper.scrape(promText, baseTime.UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		found := false
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "http_request_duration_seconds" {
				found = true
				assert.Equal(t, pmetric.MetricTypeHistogram, m.Type())
				assert.Equal(t, 1, m.Histogram().DataPoints().Len())

				dp := m.Histogram().DataPoints().At(0)
				assert.Equal(t, uint64(2500), dp.Count())
				assert.Equal(t, float64(5000), dp.Sum())
				assert.Equal(t, []float64{0.05, 0.5, 1}, dp.ExplicitBounds().AsRaw())
				// Bucket counts are delta: 1000, 500, 500, 500
				assert.Equal(t, []uint64{1000, 500, 500, 500}, dp.BucketCounts().AsRaw())
			}
		}
		assert.True(t, found, "http_request_duration_seconds metric not found")
	})
}

// TestSummaryMetricsWithSynctest tests summary metric processing.
func TestSummaryMetricsWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		promText := `
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 5
rpc_duration_seconds{quantile="0.99"} 8
rpc_duration_seconds_sum 5000
rpc_duration_seconds_count 1000
`
		baseTime := time.Now()
		result := scraper.scrape(promText, baseTime.UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		found := false
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "rpc_duration_seconds" {
				found = true
				assert.Equal(t, pmetric.MetricTypeSummary, m.Type())
				assert.Equal(t, 1, m.Summary().DataPoints().Len())

				dp := m.Summary().DataPoints().At(0)
				assert.Equal(t, uint64(1000), dp.Count())
				assert.Equal(t, float64(5000), dp.Sum())
				assert.Equal(t, 3, dp.QuantileValues().Len())
			}
		}
		assert.True(t, found, "rpc_duration_seconds metric not found")
	})
}

// TestResourceAttributesWithSynctest verifies resource attributes are set correctly.
func TestResourceAttributesWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "my_job", "myhost:9090")

		promText := `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19
`
		baseTime := time.Now()
		result := scraper.scrape(promText, baseTime.UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)

		attrs := rm.Resource().Attributes()
		serviceName, ok := attrs.Get("service.name")
		assert.True(t, ok, "service.name attribute missing")
		assert.Equal(t, "my_job", serviceName.AsString())

		instanceID, ok := attrs.Get("service.instance.id")
		assert.True(t, ok, "service.instance.id attribute missing")
		assert.Equal(t, "myhost:9090", instanceID.AsString())
	})
}

// TestTimestampsWithSynctest verifies that timestamps are set correctly.
func TestTimestampsWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		promText := `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19
`
		scrapeTime := time.Now()
		result := scraper.scrape(promText, scrapeTime.UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "go_threads" {
				dp := m.Gauge().DataPoints().At(0)
				expectedTs := pcommon.NewTimestampFromTime(scrapeTime.Truncate(time.Millisecond))
				assert.Equal(t, expectedTs, dp.Timestamp(), "timestamp should match scrape time")
			}
		}
	})
}

// TestMultipleMetricTypesWithSynctest tests processing of multiple metric types in one scrape.
func TestMultipleMetricTypesWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		result := scraper.scrape(testTarget1Page1, time.Now().UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()

		// Count metric types found
		var gaugeCount, counterCount, histCount, summaryCount int
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			switch m.Type() {
			case pmetric.MetricTypeGauge:
				gaugeCount++
			case pmetric.MetricTypeSum:
				counterCount++
			case pmetric.MetricTypeHistogram:
				histCount++
			case pmetric.MetricTypeSummary:
				summaryCount++
			}
		}

		// We expect at least one of each type from testTarget1Page1
		assert.GreaterOrEqual(t, gaugeCount, 1, "expected at least 1 gauge metric")
		assert.GreaterOrEqual(t, counterCount, 1, "expected at least 1 counter metric")
		assert.GreaterOrEqual(t, histCount, 1, "expected at least 1 histogram metric")
		assert.GreaterOrEqual(t, summaryCount, 1, "expected at least 1 summary metric")
	})
}

// verifyFirstScrapeMetrics verifies the metrics from the first scrape of testTarget1Page1.
func verifyFirstScrapeMetrics(t *testing.T, md pmetric.Metrics) {
	t.Helper()

	require.Greater(t, md.ResourceMetrics().Len(), 0)
	rm := md.ResourceMetrics().At(0)
	require.Greater(t, rm.ScopeMetrics().Len(), 0)

	metrics := rm.ScopeMetrics().At(0).Metrics()

	// Look for go_threads gauge
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if m.Name() == "go_threads" {
			assert.Equal(t, pmetric.MetricTypeGauge, m.Type())
			assert.Equal(t, float64(19), m.Gauge().DataPoints().At(0).DoubleValue())
		}
	}
}

// verifySecondScrapeMetrics verifies the metrics from the second scrape of testTarget1Page2.
func verifySecondScrapeMetrics(t *testing.T, md pmetric.Metrics) {
	t.Helper()

	require.Greater(t, md.ResourceMetrics().Len(), 0)
	rm := md.ResourceMetrics().At(0)
	require.Greater(t, rm.ScopeMetrics().Len(), 0)

	metrics := rm.ScopeMetrics().At(0).Metrics()

	// Look for go_threads gauge with updated value
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if m.Name() == "go_threads" {
			assert.Equal(t, pmetric.MetricTypeGauge, m.Type())
			assert.Equal(t, float64(18), m.Gauge().DataPoints().At(0).DoubleValue())
		}
	}
}

// TestEmptyScrapeWithSynctest tests handling of empty scrape data.
func TestEmptyScrapeWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		result := scraper.scrape("", time.Now().UnixMilli())
		// Empty scrape should not produce an error, just no metrics
		require.NoError(t, result.scrapeError)
	})
}

// TestMetricWithLabelsWithSynctest tests metrics with various label combinations.
func TestMetricWithLabelsWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		promText := `
# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="get",code="200",path="/api/v1"} 100
http_requests_total{method="post",code="201",path="/api/v1"} 50
http_requests_total{method="get",code="404",path="/api/v2"} 10
`
		result := scraper.scrape(promText, time.Now().UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "http_requests_total" {
				assert.Equal(t, 3, m.Sum().DataPoints().Len(), "expected 3 data points with different label combinations")

				// Verify each data point has the expected attributes
				for j := 0; j < m.Sum().DataPoints().Len(); j++ {
					dp := m.Sum().DataPoints().At(j)
					_, hasMethod := dp.Attributes().Get("method")
					_, hasCode := dp.Attributes().Get("code")
					_, hasPath := dp.Attributes().Get("path")
					assert.True(t, hasMethod, "data point missing 'method' attribute")
					assert.True(t, hasCode, "data point missing 'code' attribute")
					assert.True(t, hasPath, "data point missing 'path' attribute")
				}
			}
		}
	})
}

// TestScrapeIntervalSimulationWithSynctest simulates multiple scrapes at regular intervals.
func TestScrapeIntervalSimulationWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		scrapeInterval := 15 * time.Second
		numScrapes := 5

		for i := 0; i < numScrapes; i++ {
			scrapeTime := time.Now()
			promText := "# HELP request_count Total request count\n# TYPE request_count counter\nrequest_count " + strconv.Itoa(100*(i+1))

			result := scraper.scrape(promText, scrapeTime.UnixMilli())
			require.NoError(t, result.scrapeError, "scrape %d failed", i)

			if i < numScrapes-1 {
				time.Sleep(scrapeInterval)
			}
		}

		// Verify we collected metrics from all scrapes
		allMetrics := scraper.allMetrics()
		assert.Len(t, allMetrics, numScrapes, "expected %d scrape results", numScrapes)
	})
}

// TestTarget1ScenarioWithSynctest mirrors the target1 scenario from TestCoreMetricsEndToEnd.
// It tests: 3 successful scrapes with 200 status codes, testing counter, gauge, histogram, and summary.
func TestTarget1ScenarioWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "target1", "localhost:8080")

		// Page 1: Initial values
		baseTime := time.Now()
		result1 := scraper.scrape(testTarget1Page1, baseTime.UnixMilli())
		require.NoError(t, result1.scrapeError, "first scrape failed")

		// Advance time by 15 seconds
		time.Sleep(15 * time.Second)

		// Page 2: Updated values (counter increased, gauge changed)
		result2 := scraper.scrape(testTarget1Page2, time.Now().UnixMilli())
		require.NoError(t, result2.scrapeError, "second scrape failed")

		// Advance time
		time.Sleep(15 * time.Second)

		// Page 3: Values reset (counter decreased - simulates process restart)
		const testTarget1Page3 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 99
http_requests_total{method="post",code="400"} 3

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 900
http_request_duration_seconds_bucket{le="0.5"} 1400
http_request_duration_seconds_bucket{le="1"} 1900
http_request_duration_seconds_bucket{le="+Inf"} 2400
http_request_duration_seconds_sum 4900
http_request_duration_seconds_count 2400

# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.01"} 1
rpc_duration_seconds{quantile="0.9"} 4
rpc_duration_seconds{quantile="0.99"} 6
rpc_duration_seconds_sum 4900
rpc_duration_seconds_count 900
`
		result3 := scraper.scrape(testTarget1Page3, time.Now().UnixMilli())
		require.NoError(t, result3.scrapeError, "third scrape failed")

		// Verify we have all 3 scrapes
		allMetrics := scraper.allMetrics()
		require.Len(t, allMetrics, 3, "expected 3 scrape results")

		// Verify first scrape has correct go_threads value
		verifyGaugeValue(t, allMetrics[0], "go_threads", 19)

		// Verify second scrape has updated go_threads value
		verifyGaugeValue(t, allMetrics[1], "go_threads", 18)

		// Verify third scrape has reset go_threads value
		verifyGaugeValue(t, allMetrics[2], "go_threads", 16)
	})
}

// TestTarget2NewSeriesAppearingWithSynctest mirrors target2 from TestCoreMetricsEndToEnd.
// It tests new label combinations appearing mid-scrape sequence.
func TestTarget2NewSeriesAppearingWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "target2", "localhost:8080")

		// Page 1: Initial series
		const page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 10
http_requests_total{method="post",code="400"} 50
`
		baseTime := time.Now()
		result1 := scraper.scrape(page1, baseTime.UnixMilli())
		require.NoError(t, result1.scrapeError, "first scrape failed")

		time.Sleep(15 * time.Second)

		// Page 2: New series appears (code="300")
		const page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="300"} 3
http_requests_total{method="post",code="400"} 60
`
		result2 := scraper.scrape(page2, time.Now().UnixMilli())
		require.NoError(t, result2.scrapeError, "second scrape failed")

		// Verify we have both scrapes
		allMetrics := scraper.allMetrics()
		require.Len(t, allMetrics, 2, "expected 2 scrape results")

		// Verify first scrape has 2 http_requests_total data points
		verifyCounterDataPointCount(t, allMetrics[0], "http_requests_total", 2)

		// Verify second scrape has 3 http_requests_total data points (new code="300" added)
		verifyCounterDataPointCount(t, allMetrics[1], "http_requests_total", 3)
	})
}

// TestHistogramBucketDeltasWithSynctest verifies histogram bucket counts are properly delta-converted.
func TestHistogramBucketDeltasWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		// Prometheus histograms report cumulative bucket counts.
		// OTel histograms expect delta bucket counts.
		const promText = `
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.1"} 100
http_request_duration_seconds_bucket{le="0.5"} 150
http_request_duration_seconds_bucket{le="1"} 200
http_request_duration_seconds_bucket{le="+Inf"} 250
http_request_duration_seconds_sum 500
http_request_duration_seconds_count 250
`
		result := scraper.scrape(promText, time.Now().UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "http_request_duration_seconds" {
				dp := m.Histogram().DataPoints().At(0)
				// Buckets should be delta: 100, 50, 50, 50 (not cumulative: 100, 150, 200, 250)
				assert.Equal(t, []uint64{100, 50, 50, 50}, dp.BucketCounts().AsRaw(),
					"bucket counts should be delta (non-cumulative)")
				assert.Equal(t, uint64(250), dp.Count())
				assert.Equal(t, float64(500), dp.Sum())
			}
		}
	})
}

// TestSummaryQuantilesWithSynctest verifies summary quantile values are properly extracted.
func TestSummaryQuantilesWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		const promText = `
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{quantile="0.5"} 0.05
rpc_duration_seconds{quantile="0.9"} 0.1
rpc_duration_seconds{quantile="0.99"} 0.2
rpc_duration_seconds_sum 1000
rpc_duration_seconds_count 10000
`
		result := scraper.scrape(promText, time.Now().UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "rpc_duration_seconds" {
				dp := m.Summary().DataPoints().At(0)
				assert.Equal(t, uint64(10000), dp.Count())
				assert.Equal(t, float64(1000), dp.Sum())
				assert.Equal(t, 3, dp.QuantileValues().Len())

				// Check quantile values
				quantiles := make(map[float64]float64)
				for j := 0; j < dp.QuantileValues().Len(); j++ {
					qv := dp.QuantileValues().At(j)
					quantiles[qv.Quantile()] = qv.Value()
				}
				assert.Equal(t, 0.05, quantiles[0.5], "p50 quantile mismatch")
				assert.Equal(t, 0.1, quantiles[0.9], "p90 quantile mismatch")
				assert.Equal(t, 0.2, quantiles[0.99], "p99 quantile mismatch")
			}
		}
	})
}

// TestMultipleLabelsOnSameMetricWithSynctest tests metrics with multiple label sets.
func TestMultipleLabelsOnSameMetricWithSynctest(t *testing.T) {
	synctest.Run(func() {
		scraper := newTestScraper(t, "test_job", "localhost:8080")

		const promText = `
# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 100
http_requests_total{method="GET",status="404"} 10
http_requests_total{method="POST",status="200"} 50
http_requests_total{method="POST",status="500"} 5
`
		result := scraper.scrape(promText, time.Now().UnixMilli())
		require.NoError(t, result.scrapeError)

		require.Greater(t, result.metrics.ResourceMetrics().Len(), 0)
		rm := result.metrics.ResourceMetrics().At(0)
		require.Greater(t, rm.ScopeMetrics().Len(), 0)

		metrics := rm.ScopeMetrics().At(0).Metrics()
		for i := 0; i < metrics.Len(); i++ {
			m := metrics.At(i)
			if m.Name() == "http_requests_total" {
				assert.Equal(t, 4, m.Sum().DataPoints().Len(),
					"expected 4 data points for different label combinations")
			}
		}
	})
}

// verifyGaugeValue is a helper to verify a gauge metric has the expected value.
func verifyGaugeValue(t *testing.T, md pmetric.Metrics, metricName string, expectedValue float64) {
	t.Helper()

	require.Greater(t, md.ResourceMetrics().Len(), 0)
	rm := md.ResourceMetrics().At(0)
	require.Greater(t, rm.ScopeMetrics().Len(), 0)

	metrics := rm.ScopeMetrics().At(0).Metrics()
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if m.Name() == metricName {
			require.Equal(t, pmetric.MetricTypeGauge, m.Type())
			require.GreaterOrEqual(t, m.Gauge().DataPoints().Len(), 1)
			assert.Equal(t, expectedValue, m.Gauge().DataPoints().At(0).DoubleValue(),
				"gauge %s has wrong value", metricName)
			return
		}
	}
	t.Errorf("metric %s not found", metricName)
}

// verifyCounterDataPointCount is a helper to verify a counter has the expected number of data points.
func verifyCounterDataPointCount(t *testing.T, md pmetric.Metrics, metricName string, expectedCount int) {
	t.Helper()

	require.Greater(t, md.ResourceMetrics().Len(), 0)
	rm := md.ResourceMetrics().At(0)
	require.Greater(t, rm.ScopeMetrics().Len(), 0)

	metrics := rm.ScopeMetrics().At(0).Metrics()
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if m.Name() == metricName {
			require.Equal(t, pmetric.MetricTypeSum, m.Type())
			assert.Equal(t, expectedCount, m.Sum().DataPoints().Len(),
				"counter %s has wrong number of data points", metricName)
			return
		}
	}
	t.Errorf("metric %s not found", metricName)
}
