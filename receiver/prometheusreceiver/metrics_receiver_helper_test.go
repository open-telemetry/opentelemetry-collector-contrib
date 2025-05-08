// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/common/promslog"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	dto "github.com/prometheus/prometheus/prompb/io/prometheus/client"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

type mockPrometheusResponse struct {
	code           int
	data           string
	useOpenMetrics bool

	useProtoBuf bool // This overrides data and useOpenMetrics above
	buf         []byte
}

type mockPrometheus struct {
	mu          sync.Mutex // mu protects the fields below.
	endpoints   map[string][]mockPrometheusResponse
	accessIndex map[string]*atomic.Int32
	wg          *sync.WaitGroup
	srv         *httptest.Server
}

func newMockPrometheus(endpoints map[string][]mockPrometheusResponse) *mockPrometheus {
	accessIndex := make(map[string]*atomic.Int32)
	wg := &sync.WaitGroup{}
	wg.Add(len(endpoints))
	for k := range endpoints {
		accessIndex[k] = &atomic.Int32{}
	}
	mp := &mockPrometheus{
		wg:          wg,
		accessIndex: accessIndex,
		endpoints:   endpoints,
	}
	srv := httptest.NewServer(mp)
	mp.srv = srv
	return mp
}

func (mp *mockPrometheus) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	iptr, ok := mp.accessIndex[req.URL.Path]
	if !ok {
		rw.WriteHeader(http.StatusNotFound)
		return
	}
	index := int(iptr.Load())
	iptr.Add(1)
	pages := mp.endpoints[req.URL.Path]
	if index >= len(pages) {
		if index == len(pages) {
			mp.wg.Done()
		}
		rw.WriteHeader(http.StatusNotFound)
		return
	}
	switch {
	case pages[index].useProtoBuf:
		rw.Header().Set("Content-Type", "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited")
	case pages[index].useOpenMetrics:
		rw.Header().Set("Content-Type", "application/openmetrics-text")
	}
	rw.WriteHeader(pages[index].code)
	if pages[index].useProtoBuf {
		_, _ = rw.Write(pages[index].buf)
	} else {
		_, _ = rw.Write([]byte(pages[index].data))
	}
}

func (mp *mockPrometheus) Close() {
	mp.srv.Close()
}

// -------------------------
// EndToEnd Test and related
// -------------------------

var (
	expectedScrapeMetricCount      = 5
	expectedExtraScrapeMetricCount = 8
)

type testData struct {
	name            string
	relabeledJob    string // Used when relabeling or honor_labels changes the target to something other than 'name'.
	pages           []mockPrometheusResponse
	attributes      pcommon.Map
	validateScrapes bool
	normalizedName  bool
	validateFunc    func(t *testing.T, td *testData, result []pmetric.ResourceMetrics)
}

// setupMockPrometheus to create a mocked prometheus based on targets, returning the server and a prometheus exporting
// config
func setupMockPrometheus(tds ...*testData) (*mockPrometheus, *PromConfig, error) {
	jobs := make([]map[string]any, 0, len(tds))
	endpoints := make(map[string][]mockPrometheusResponse)
	metricPaths := make([]string, len(tds))
	for i, t := range tds {
		metricPath := fmt.Sprintf("/%s/metrics", t.name)
		endpoints[metricPath] = t.pages
		metricPaths[i] = metricPath
	}
	mp := newMockPrometheus(endpoints)
	u, _ := url.Parse(mp.srv.URL)
	for i := 0; i < len(tds); i++ {
		job := make(map[string]any)
		job["job_name"] = tds[i].name
		job["metrics_path"] = metricPaths[i]
		job["scrape_interval"] = "1s"
		job["scrape_timeout"] = "500ms"
		job["static_configs"] = []map[string]any{{"targets": []string{u.Host}}}
		jobs = append(jobs, job)
	}
	if len(jobs) != len(tds) {
		log.Fatal("len(jobs) != len(targets), make sure job names are unique")
	}
	configP := make(map[string]any)
	configP["scrape_configs"] = jobs
	cfg, err := yaml.Marshal(&configP)
	if err != nil {
		return mp, nil, err
	}
	// update attributes value (will use for validation)
	l := []labels.Label{{Name: "__scheme__", Value: "http"}}
	for _, t := range tds {
		t.attributes = internal.CreateResource(t.name, u.Host, labels.New(l...)).Attributes()
	}
	pCfg, err := promcfg.Load(string(cfg), promslog.NewNopLogger())
	return mp, (*PromConfig)(pCfg), err
}

func waitForScrapeResults(t *testing.T, targets []*testData, cms *consumertest.MetricsSink) {
	assert.Eventually(t, func() bool {
		// This is the receiver's pov as to what should have been collected from the server
		metrics := cms.AllMetrics()
		pResults := splitMetricsByTarget(metrics)
		for _, target := range targets {
			want := 0
			name := target.name
			if target.relabeledJob != "" {
				name = target.relabeledJob
			}
			scrapes := pResults[name]
			// count the number of pages we expect for a target endpoint
			for _, p := range target.pages {
				if p.code != 404 {
					// only count target pages that are not 404, matching mock ServerHTTP func response logic
					want++
				}
			}
			if len(scrapes) < want {
				// If we don't have enough scrapes yet lets return false and wait for another tick
				return false
			}
		}
		return true
	}, 30*time.Second, 500*time.Millisecond)
}

func verifyNumValidScrapeResults(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	want := 0
	for _, p := range td.pages {
		if p.code == 200 {
			want++
		}
	}
	require.LessOrEqual(t, want, len(resourceMetrics), "want at least %d valid scrapes, but got %d", want, len(resourceMetrics))
}

func verifyNumTotalScrapeResults(t *testing.T, td *testData, resourceMetrics []pmetric.ResourceMetrics) {
	want := 0
	for _, p := range td.pages {
		if p.code == 200 || p.code == 500 {
			want++
		}
	}
	require.LessOrEqual(t, want, len(resourceMetrics), "want at least %d total scrapes, but got %d", want, len(resourceMetrics))
}

func getMetrics(rm pmetric.ResourceMetrics) []pmetric.Metric {
	var metrics []pmetric.Metric
	ilms := rm.ScopeMetrics()
	for j := 0; j < ilms.Len(); j++ {
		metricSlice := ilms.At(j).Metrics()
		for i := 0; i < metricSlice.Len(); i++ {
			metrics = append(metrics, metricSlice.At(i))
		}
	}
	return metrics
}

func metricsCount(resourceMetric pmetric.ResourceMetrics) int {
	metricsCount := 0
	ilms := resourceMetric.ScopeMetrics()
	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j)
		metricsCount += ilm.Metrics().Len()
	}
	return metricsCount
}

func getValidScrapes(t *testing.T, rms []pmetric.ResourceMetrics, target *testData) []pmetric.ResourceMetrics {
	var out []pmetric.ResourceMetrics
	// rms will include failed scrapes and scrapes that received no metrics but have internal scrape metrics, filter those out
	// for metrics retrieved with 'honor_labels: true', there will be a resource metric containing the scrape metrics, based on the scrape job config,
	// and resources containing only the retrieved metrics, without additional scrape metrics, based on the job/instance label pairs that are detected
	// during a scrape
	for i := 0; i < len(rms); i++ {
		allMetrics := getMetrics(rms[i])
		if expectedScrapeMetricCount <= len(allMetrics) && countScrapeMetrics(allMetrics, target.normalizedName) == expectedScrapeMetricCount ||
			expectedExtraScrapeMetricCount <= len(allMetrics) && countScrapeMetrics(allMetrics, target.normalizedName) == expectedExtraScrapeMetricCount {
			if isFirstFailedScrape(allMetrics, target.normalizedName) {
				continue
			}
			assertUp(t, 1, allMetrics)
			out = append(out, rms[i])
		} else {
			if isScrapeConfigResource(rms[i], target) {
				assertUp(t, 0, allMetrics)
			} else {
				out = append(out, rms[i])
			}
		}
	}
	return out
}

func isScrapeConfigResource(rms pmetric.ResourceMetrics, target *testData) bool {
	targetJobName, ok := target.attributes.Get(semconv.AttributeServiceName)
	if !ok {
		return false
	}
	targetInstanceID, ok := target.attributes.Get(semconv.AttributeServiceInstanceID)
	if !ok {
		return false
	}

	resourceJobName, ok := rms.Resource().Attributes().Get(semconv.AttributeServiceName)
	if !ok {
		return false
	}
	resourceInstanceID, ok := rms.Resource().Attributes().Get(semconv.AttributeServiceInstanceID)
	if !ok {
		return false
	}

	return resourceJobName.AsString() == targetJobName.AsString() && resourceInstanceID.AsString() == targetInstanceID.AsString()
}

func isFirstFailedScrape(metrics []pmetric.Metric, normalizedNames bool) bool {
	for _, m := range metrics {
		if m.Name() == "up" {
			if m.Gauge().DataPoints().At(0).DoubleValue() == 1 { // assumed up will not have multiple datapoints
				return false
			}
		}
	}

	for _, m := range metrics {
		if isDefaultMetrics(m, normalizedNames) || isExtraScrapeMetrics(m) {
			continue
		}

		switch m.Type() {
		case pmetric.MetricTypeGauge:
			for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
				if !m.Gauge().DataPoints().At(i).Flags().NoRecordedValue() {
					return false
				}
			}
		case pmetric.MetricTypeSum:
			for i := 0; i < m.Sum().DataPoints().Len(); i++ {
				if !m.Sum().DataPoints().At(i).Flags().NoRecordedValue() {
					return false
				}
			}
		case pmetric.MetricTypeExponentialHistogram:
			for i := 0; i < m.ExponentialHistogram().DataPoints().Len(); i++ {
				if !m.ExponentialHistogram().DataPoints().At(i).Flags().NoRecordedValue() {
					return false
				}
			}
		case pmetric.MetricTypeHistogram:
			for i := 0; i < m.Histogram().DataPoints().Len(); i++ {
				if !m.Histogram().DataPoints().At(i).Flags().NoRecordedValue() {
					return false
				}
			}
		case pmetric.MetricTypeSummary:
			for i := 0; i < m.Summary().DataPoints().Len(); i++ {
				if !m.Summary().DataPoints().At(i).Flags().NoRecordedValue() {
					return false
				}
			}
		case pmetric.MetricTypeEmpty:
		}
	}
	return true
}

func assertUp(t *testing.T, expected float64, metrics []pmetric.Metric) {
	for _, m := range metrics {
		if m.Name() == "up" {
			assert.Equal(t, expected, m.Gauge().DataPoints().At(0).DoubleValue()) // (assumed up will not have multiple datapoints)
			return
		}
	}
	t.Error("No 'up' metric found")
}

func countScrapeMetricsRM(got pmetric.ResourceMetrics, normalizedNames bool) int {
	n := 0
	ilms := got.ScopeMetrics()
	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j)
		for i := 0; i < ilm.Metrics().Len(); i++ {
			if isDefaultMetrics(ilm.Metrics().At(i), normalizedNames) {
				n++
			}
		}
	}
	return n
}

func countScrapeMetrics(metrics []pmetric.Metric, normalizedNames bool) int {
	n := 0
	for _, m := range metrics {
		if isDefaultMetrics(m, normalizedNames) || isExtraScrapeMetrics(m) {
			n++
		}
	}
	return n
}

func isDefaultMetrics(m pmetric.Metric, normalizedNames bool) bool {
	switch m.Name() {
	case "up", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added":
		return true

	// if normalizedNames is true, we expect unit `_seconds` to be trimmed.
	case "scrape_duration_seconds":
		return !normalizedNames
	case "scrape_duration":
		return normalizedNames
	default:
	}
	return false
}

func isExtraScrapeMetrics(m pmetric.Metric) bool {
	switch m.Name() {
	case "scrape_body_size_bytes", "scrape_sample_limit", "scrape_timeout_seconds":
		return true
	default:
		return false
	}
}

type (
	numberPointComparator          func(*testing.T, pmetric.NumberDataPoint)
	histogramPointComparator       func(*testing.T, pmetric.HistogramDataPoint)
	summaryPointComparator         func(*testing.T, pmetric.SummaryDataPoint)
	exponentialHistogramComparator func(*testing.T, pmetric.ExponentialHistogramDataPoint)
)

type dataPointExpectation struct {
	numberPointComparator          []numberPointComparator
	histogramPointComparator       []histogramPointComparator
	summaryPointComparator         []summaryPointComparator
	exponentialHistogramComparator []exponentialHistogramComparator
}

type metricChecker func(*testing.T, pmetric.Metric)

type metricExpectation struct {
	name                  string
	mtype                 pmetric.MetricType
	munit                 string
	dataPointExpectations []dataPointExpectation
	extraExpectation      metricChecker
}

// doCompare is a helper function to compare the expected metrics with the actual metrics
// name is the test name
// want is the map of expected attributes
// got is the actual metrics
// metricExpections is the list of expected metrics, exluding the internal scrape metrics
func doCompare(t *testing.T, name string, want pcommon.Map, got pmetric.ResourceMetrics, metricExpectations []metricExpectation) {
	doCompareNormalized(t, name, want, got, metricExpectations, false)
}

func doCompareNormalized(t *testing.T, name string, want pcommon.Map, got pmetric.ResourceMetrics, metricExpectations []metricExpectation, normalizedNames bool) {
	t.Run(name, func(t *testing.T) {
		assert.Equal(t, expectedScrapeMetricCount, countScrapeMetricsRM(got, normalizedNames))
		assertExpectedAttributes(t, want, got)
		assertExpectedMetrics(t, metricExpectations, got, normalizedNames, false)
	})
}

func assertExpectedAttributes(t *testing.T, want pcommon.Map, got pmetric.ResourceMetrics) {
	assert.Equal(t, want.Len(), got.Resource().Attributes().Len())
	for k, v := range want.AsRaw() {
		val, ok := got.Resource().Attributes().Get(k)
		assert.True(t, ok, "%q attribute is missing", k)
		if ok {
			assert.EqualValues(t, v, val.AsString())
		}
	}
}

func assertExpectedMetrics(t *testing.T, metricExpectations []metricExpectation, got pmetric.ResourceMetrics, normalizedNames bool, existsOnly bool) {
	var defaultExpectations []metricExpectation
	switch {
	case existsOnly:
	case normalizedNames:
		defaultExpectations = []metricExpectation{
			{
				"scrape_duration",
				pmetric.MetricTypeGauge,
				"s",
				nil,
				nil,
			},
			{
				"scrape_samples_post_metric_relabeling",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
			{
				"scrape_samples_scraped",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
			{
				"scrape_series_added",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
			{
				"up",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
		}
	case !normalizedNames:
		defaultExpectations = []metricExpectation{
			{
				"scrape_duration_seconds",
				pmetric.MetricTypeGauge,
				"s",
				nil,
				nil,
			},
			{
				"scrape_samples_post_metric_relabeling",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
			{
				"scrape_samples_scraped",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
			{
				"scrape_series_added",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
			{
				"up",
				pmetric.MetricTypeGauge,
				"",
				nil,
				nil,
			},
		}
	}

	metricExpectations = append(defaultExpectations, metricExpectations...)

	allMetrics := getMetrics(got)
	for _, me := range metricExpectations {
		id := fmt.Sprintf("name '%s' type '%s' unit '%s'", me.name, me.mtype.String(), me.munit)
		pos := -1
		for k, m := range allMetrics {
			if me.name != m.Name() || me.mtype != m.Type() || me.munit != m.Unit() {
				continue
			}

			require.Equal(t, -1, pos, "metric %s is not unique", id)
			pos = k

			for i, de := range me.dataPointExpectations {
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					for _, npc := range de.numberPointComparator {
						require.Len(t, me.dataPointExpectations, m.Gauge().DataPoints().Len(), "Expected number of data-points in Gauge metric '%s' does not match to testdata", id)
						npc(t, m.Gauge().DataPoints().At(i))
					}
				case pmetric.MetricTypeSum:
					for _, npc := range de.numberPointComparator {
						require.Len(t, me.dataPointExpectations, m.Sum().DataPoints().Len(), "Expected number of data-points in Sum metric '%s' does not match to testdata", id)
						npc(t, m.Sum().DataPoints().At(i))
					}
				case pmetric.MetricTypeHistogram:
					for _, hpc := range de.histogramPointComparator {
						require.Len(t, me.dataPointExpectations, m.Histogram().DataPoints().Len(), "Expected number of data-points in Histogram metric '%s' does not match to testdata", id)
						hpc(t, m.Histogram().DataPoints().At(i))
					}
				case pmetric.MetricTypeSummary:
					for _, spc := range de.summaryPointComparator {
						require.Len(t, me.dataPointExpectations, m.Summary().DataPoints().Len(), "Expected number of data-points in Summary metric '%s' does not match to testdata", id)
						spc(t, m.Summary().DataPoints().At(i))
					}
				case pmetric.MetricTypeExponentialHistogram:
					for _, ehc := range de.exponentialHistogramComparator {
						require.Len(t, me.dataPointExpectations, m.ExponentialHistogram().DataPoints().Len(), "Expected number of data-points in Exponential Histogram metric '%s' does not match to testdata", id)
						ehc(t, m.ExponentialHistogram().DataPoints().At(i))
					}
				case pmetric.MetricTypeEmpty:
				}
			}

			if me.extraExpectation != nil {
				me.extraExpectation(t, m)
			}
		}
		require.GreaterOrEqual(t, pos, 0, "expected metric %s is not present", id)
		allMetrics = append(allMetrics[:pos], allMetrics[pos+1:]...)
	}

	if existsOnly {
		return
	}

	remainingMetrics := []string{}
	for _, m := range allMetrics {
		remainingMetrics = append(remainingMetrics, fmt.Sprintf("%s(%s,%s)", m.Name(), m.Type().String(), m.Unit()))
	}

	require.Empty(t, allMetrics, "not all metrics were validated: %v", strings.Join(remainingMetrics, ", "))
}

func compareMetricIsMonotonic(isMonotonic bool) metricChecker {
	return func(t *testing.T, metric pmetric.Metric) {
		assert.Equal(t, pmetric.MetricTypeSum.String(), metric.Type().String(), "IsMonotonic only exists for sums")
		assert.Equal(t, isMonotonic, metric.Sum().IsMonotonic(), "IsMonotonic does not match")
	}
}

func compareAttributes(attributes map[string]string) numberPointComparator {
	return func(t *testing.T, numberDataPoint pmetric.NumberDataPoint) {
		req := assert.Equal(t, len(attributes), numberDataPoint.Attributes().Len(), "Attributes length do not match")
		if req {
			for k, v := range attributes {
				val, ok := numberDataPoint.Attributes().Get(k)
				require.True(t, ok)
				assert.Equal(t, v, val.AsString(), "Attributes do not match")
			}
		}
	}
}

func compareSummaryAttributes(attributes map[string]string) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint pmetric.SummaryDataPoint) {
		req := assert.Equal(t, len(attributes), summaryDataPoint.Attributes().Len(), "Summary attributes length do not match")
		if req {
			for k, v := range attributes {
				val, ok := summaryDataPoint.Attributes().Get(k)
				require.True(t, ok)
				assert.Equal(t, v, val.AsString(), "Summary attributes value do not match")
			}
		}
	}
}

func assertAttributesAbsent() numberPointComparator {
	return func(t *testing.T, numberDataPoint pmetric.NumberDataPoint) {
		assert.Equal(t, 0, numberDataPoint.Attributes().Len(), "Attributes length should be 0")
	}
}

func compareHistogramAttributes(attributes map[string]string) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint pmetric.HistogramDataPoint) {
		req := assert.Equal(t, len(attributes), histogramDataPoint.Attributes().Len(), "Histogram attributes length do not match")
		if req {
			for k, v := range attributes {
				val, ok := histogramDataPoint.Attributes().Get(k)
				require.True(t, ok)
				assert.Equal(t, v, val.AsString(), "Histogram attributes value do not match")
			}
		}
	}
}

func assertNumberPointFlagNoRecordedValue() numberPointComparator {
	return func(t *testing.T, numberDataPoint pmetric.NumberDataPoint) {
		assert.True(t, numberDataPoint.Flags().NoRecordedValue(),
			"Datapoint flag for staleness marker not found as expected")
	}
}

func assertHistogramPointFlagNoRecordedValue() histogramPointComparator {
	return func(t *testing.T, histogramDataPoint pmetric.HistogramDataPoint) {
		assert.True(t, histogramDataPoint.Flags().NoRecordedValue(),
			"Datapoint flag for staleness marker not found as expected")
	}
}

func assertExponentialHistogramPointFlagNoRecordedValue() exponentialHistogramComparator {
	return func(t *testing.T, histogramDataPoint pmetric.ExponentialHistogramDataPoint) {
		assert.True(t, histogramDataPoint.Flags().NoRecordedValue(),
			"Datapoint flag for staleness marker not found as expected")
	}
}

func assertSummaryPointFlagNoRecordedValue() summaryPointComparator {
	return func(t *testing.T, summaryDataPoint pmetric.SummaryDataPoint) {
		assert.True(t, summaryDataPoint.Flags().NoRecordedValue(),
			"Datapoint flag for staleness marker not found as expected")
	}
}

func compareStartTimestamp(startTimeStamp pcommon.Timestamp) numberPointComparator {
	return func(t *testing.T, numberDataPoint pmetric.NumberDataPoint) {
		assert.Equal(t, startTimeStamp.String(), numberDataPoint.StartTimestamp().String(), "Start-Timestamp does not match")
	}
}

func compareTimestamp(timeStamp pcommon.Timestamp) numberPointComparator {
	return func(t *testing.T, numberDataPoint pmetric.NumberDataPoint) {
		assert.Equal(t, timeStamp.String(), numberDataPoint.Timestamp().String(), "Timestamp does not match")
	}
}

func compareHistogramTimestamp(timeStamp pcommon.Timestamp) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint pmetric.HistogramDataPoint) {
		assert.Equal(t, timeStamp.String(), histogramDataPoint.Timestamp().String(), "Histogram Timestamp does not match")
	}
}

func compareHistogramStartTimestamp(timeStamp pcommon.Timestamp) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint pmetric.HistogramDataPoint) {
		assert.Equal(t, timeStamp.String(), histogramDataPoint.StartTimestamp().String(), "Histogram Start-Timestamp does not match")
	}
}

func compareSummaryTimestamp(timeStamp pcommon.Timestamp) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint pmetric.SummaryDataPoint) {
		assert.Equal(t, timeStamp.String(), summaryDataPoint.Timestamp().String(), "Summary Timestamp does not match")
	}
}

func compareSummaryStartTimestamp(timeStamp pcommon.Timestamp) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint pmetric.SummaryDataPoint) {
		assert.Equal(t, timeStamp.String(), summaryDataPoint.StartTimestamp().String(), "Summary Start-Timestamp does not match")
	}
}

func compareDoubleValue(doubleVal float64) numberPointComparator {
	return func(t *testing.T, numberDataPoint pmetric.NumberDataPoint) {
		assert.Equal(t, doubleVal, numberDataPoint.DoubleValue(), "Metric double value does not match")
	}
}

func assertNormalNan() numberPointComparator {
	return func(t *testing.T, numberDataPoint pmetric.NumberDataPoint) {
		assert.Equal(t, value.NormalNaN, math.Float64bits(numberDataPoint.DoubleValue()),
			"Metric double value is not normalNaN as expected")
	}
}

func compareHistogram(count uint64, sum float64, upperBounds []float64, buckets []uint64) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint pmetric.HistogramDataPoint) {
		assert.Equal(t, count, histogramDataPoint.Count(), "Histogram count value does not match")
		assert.Equal(t, sum, histogramDataPoint.Sum(), "Histogram sum value does not match")
		assert.Equal(t, upperBounds, histogramDataPoint.ExplicitBounds().AsRaw(), "Histogram upper bounds values do not match")
		assert.Equal(t, buckets, histogramDataPoint.BucketCounts().AsRaw(), "Histogram bucket count values do not match")
	}
}

func compareExponentialHistogram(scale int32, count uint64, sum float64, zeroCount uint64, negativeOffset int32, negativeBuckets []uint64, positiveOffset int32, positiveBuckets []uint64) exponentialHistogramComparator {
	return func(t *testing.T, exponentialHistogramDataPoint pmetric.ExponentialHistogramDataPoint) {
		assert.Equal(t, scale, exponentialHistogramDataPoint.Scale(), "Exponential Histogram scale value does not match")
		assert.Equal(t, count, exponentialHistogramDataPoint.Count(), "Exponential Histogram count value does not match")
		assert.Equal(t, sum, exponentialHistogramDataPoint.Sum(), "Exponential Histogram sum value does not match")
		assert.Equal(t, zeroCount, exponentialHistogramDataPoint.ZeroCount(), "Exponential Histogram zero count value does not match")
		assert.Equal(t, negativeOffset, exponentialHistogramDataPoint.Negative().Offset(), "Exponential Histogram negative offset value does not match")
		assert.Equal(t, negativeBuckets, exponentialHistogramDataPoint.Negative().BucketCounts().AsRaw(), "Exponential Histogram negative bucket count values do not match")
		assert.Equal(t, positiveOffset, exponentialHistogramDataPoint.Positive().Offset(), "Exponential Histogram positive offset value does not match")
		assert.Equal(t, positiveBuckets, exponentialHistogramDataPoint.Positive().BucketCounts().AsRaw(), "Exponential Histogram positive bucket count values do not match")
	}
}

func compareSummary(count uint64, sum float64, quantiles [][]float64) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint pmetric.SummaryDataPoint) {
		assert.Equal(t, count, summaryDataPoint.Count(), "Summary count value does not match")
		assert.Equal(t, sum, summaryDataPoint.Sum(), "Summary sum value does not match")
		req := assert.Equal(t, len(quantiles), summaryDataPoint.QuantileValues().Len())
		if req {
			for i := 0; i < summaryDataPoint.QuantileValues().Len(); i++ {
				assert.Equal(t, quantiles[i][0], summaryDataPoint.QuantileValues().At(i).Quantile(),
					"Summary quantile do not match")
				if math.IsNaN(quantiles[i][1]) {
					assert.Equal(t, value.NormalNaN, math.Float64bits(summaryDataPoint.QuantileValues().At(i).Value()),
						"Summary quantile value is not normalNaN as expected")
				} else {
					assert.Equal(t, quantiles[i][1], summaryDataPoint.QuantileValues().At(i).Value(),
						"Summary quantile values do not match")
				}
			}
		}
	}
}

// starts prometheus receiver with custom config, retrieves metrics from MetricsSink
func testComponent(t *testing.T, targets []*testData, alterConfig func(*Config), cfgMuts ...func(*PromConfig)) {
	ctx := context.Background()
	mp, cfg, err := setupMockPrometheus(targets...)
	for _, cfgMut := range cfgMuts {
		cfgMut(cfg)
	}
	require.NoErrorf(t, err, "Failed to create Prometheus config: %v", err)
	defer mp.Close()

	config := &Config{
		PrometheusConfig:     cfg,
		StartTimeMetricRegex: "",
	}
	if alterConfig != nil {
		alterConfig(config)
	}

	cms := new(consumertest.MetricsSink)
	receiver := newPrometheusReceiver(receivertest.NewNopSettings(metadata.Type), config, cms)
	receiver.skipOffsetting = true

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
	assert.GreaterOrEqualf(t, lres, lep, "want at least %d targets, but got %v\n", lep, lres)

	// loop to validate outputs for each targets
	// Stop once we have evaluated all expected results, any others are superfluous.
	for _, target := range targets[:lep] {
		t.Run(target.name, func(t *testing.T) {
			name := target.name
			if target.relabeledJob != "" {
				name = target.relabeledJob
			}
			scrapes := pResults[name]
			if !target.validateScrapes {
				scrapes = getValidScrapes(t, pResults[name], target)
			}
			target.validateFunc(t, target, scrapes)
		})
	}
}

// flattenTargets takes a map of jobs to target and flattens to a list of targets
func flattenTargets(targets map[string][]*scrape.Target) []*scrape.Target {
	var flatTargets []*scrape.Target
	for _, target := range targets {
		flatTargets = append(flatTargets, target...)
	}
	return flatTargets
}

func splitMetricsByTarget(metrics []pmetric.Metrics) map[string][]pmetric.ResourceMetrics {
	pResults := make(map[string][]pmetric.ResourceMetrics)
	for _, md := range metrics {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			name, _ := rms.At(i).Resource().Attributes().Get("service.name")
			pResults[name.AsString()] = append(pResults[name.AsString()], rms.At(i))
		}
	}
	return pResults
}

func getTS(ms pmetric.MetricSlice) pcommon.Timestamp {
	if ms.Len() == 0 {
		return 0
	}
	m := ms.At(0)
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return m.Gauge().DataPoints().At(0).Timestamp()
	case pmetric.MetricTypeSum:
		return m.Sum().DataPoints().At(0).Timestamp()
	case pmetric.MetricTypeHistogram:
		return m.Histogram().DataPoints().At(0).Timestamp()
	case pmetric.MetricTypeSummary:
		return m.Summary().DataPoints().At(0).Timestamp()
	case pmetric.MetricTypeExponentialHistogram:
		return m.ExponentialHistogram().DataPoints().At(0).Timestamp()
	case pmetric.MetricTypeEmpty:
	}
	return 0
}

func prometheusMetricFamilyToProtoBuf(t *testing.T, buffer *bytes.Buffer, metricFamily *dto.MetricFamily) *bytes.Buffer {
	if buffer == nil {
		buffer = &bytes.Buffer{}
	}

	data, err := proto.Marshal(metricFamily)
	require.NoError(t, err)

	varintBuf := make([]byte, binary.MaxVarintLen32)
	varintLength := binary.PutUvarint(varintBuf, uint64(len(data)))

	_, err = buffer.Write(varintBuf[:varintLength])
	require.NoError(t, err)
	_, err = buffer.Write(data)
	require.NoError(t, err)

	return buffer
}
