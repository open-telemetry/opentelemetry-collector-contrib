// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	gokitlog "github.com/go-kit/log"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"gopkg.in/yaml.v2"
)

type mockPrometheusResponse struct {
	code int
	data string
}

type mockPrometheus struct {
	mu             sync.Mutex // mu protects the fields below.
	endpoints      map[string][]mockPrometheusResponse
	accessIndex    map[string]*int32
	wg             *sync.WaitGroup
	srv            *httptest.Server
	useOpenMetrics bool
}

func newMockPrometheus(endpoints map[string][]mockPrometheusResponse, openMetricsContentType bool) *mockPrometheus {
	accessIndex := make(map[string]*int32)
	wg := &sync.WaitGroup{}
	wg.Add(len(endpoints))
	for k := range endpoints {
		v := int32(0)
		accessIndex[k] = &v
	}
	mp := &mockPrometheus{
		wg:             wg,
		accessIndex:    accessIndex,
		endpoints:      endpoints,
		useOpenMetrics: openMetricsContentType,
	}
	srv := httptest.NewServer(mp)
	mp.srv = srv
	return mp
}

func (mp *mockPrometheus) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.useOpenMetrics {
		rw.Header().Set("Content-Type", "application/openmetrics-text")
	}
	iptr, ok := mp.accessIndex[req.URL.Path]
	if !ok {
		rw.WriteHeader(404)
		return
	}
	index := int(*iptr)
	atomic.AddInt32(iptr, 1)
	pages := mp.endpoints[req.URL.Path]
	if index >= len(pages) {
		if index == len(pages) {
			mp.wg.Done()
		}
		rw.WriteHeader(404)
		return
	}
	rw.WriteHeader(pages[index].code)
	_, _ = rw.Write([]byte(pages[index].data))
}

func (mp *mockPrometheus) Close() {
	mp.srv.Close()
}

// -------------------------
// EndToEnd Test and related
// -------------------------

var (
	expectedScrapeMetricCount = 5
)

type testData struct {
	name         string
	pages        []mockPrometheusResponse
	attributes   pdata.AttributeMap
	validateFunc func(t *testing.T, td *testData, result []*pdata.ResourceMetrics)
}

// setupMockPrometheus to create a mocked prometheus based on targets, returning the server and a prometheus exporting
// config
func setupMockPrometheus(openMetricsContentType bool, tds ...*testData) (*mockPrometheus, *promcfg.Config, error) {
	jobs := make([]map[string]interface{}, 0, len(tds))
	endpoints := make(map[string][]mockPrometheusResponse)
	metricPaths := make([]string, 0)
	for _, t := range tds {
		metricPath := fmt.Sprintf("/%s/metrics", t.name)
		endpoints[metricPath] = t.pages
		metricPaths = append(metricPaths, metricPath)
	}
	mp := newMockPrometheus(endpoints, openMetricsContentType)
	u, _ := url.Parse(mp.srv.URL)
	host, port, _ := net.SplitHostPort(u.Host)
	for i := 0; i < len(tds); i++ {
		job := make(map[string]interface{})
		job["job_name"] = tds[i].name
		job["metrics_path"] = metricPaths[i]
		job["scrape_interval"] = "1s"
		job["static_configs"] = []map[string]interface{}{{"targets": []string{u.Host}}}
		jobs = append(jobs, job)
	}
	if len(jobs) != len(tds) {
		log.Fatal("len(jobs) != len(targets), make sure job names are unique")
	}
	configP := make(map[string]interface{})
	configP["scrape_configs"] = jobs
	cfg, err := yaml.Marshal(&configP)
	if err != nil {
		return mp, nil, err
	}
	// update attributes value (will use for validation)
	for _, t := range tds {
		t.attributes = pdata.NewAttributeMap()
		t.attributes.Insert("service.name", pdata.NewAttributeValueString(t.name))
		t.attributes.Insert("host.name", pdata.NewAttributeValueString(host))
		t.attributes.Insert("job", pdata.NewAttributeValueString(t.name))
		t.attributes.Insert("instance", pdata.NewAttributeValueString(u.Host))
		t.attributes.Insert("port", pdata.NewAttributeValueString(port))
		t.attributes.Insert("scheme", pdata.NewAttributeValueString("http"))
	}
	pCfg, err := promcfg.Load(string(cfg), false, gokitlog.NewNopLogger())
	return mp, pCfg, err
}

func verifyNumScrapeResults(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	want := 0
	for _, p := range td.pages {
		if p.code == 200 {
			want++
		}
	}
	require.Equal(t, want, len(resourceMetrics), "want %d valid scrapes, but got %d", want, len(resourceMetrics))
}

func getMetrics(rm *pdata.ResourceMetrics) []*pdata.Metric {
	metrics := make([]*pdata.Metric, 0)
	ilms := rm.InstrumentationLibraryMetrics()
	for j := 0; j < ilms.Len(); j++ {
		metricSlice := ilms.At(j).Metrics()
		for i := 0; i < metricSlice.Len(); i++ {
			m := metricSlice.At(i)
			metrics = append(metrics, &m)
		}
	}
	return metrics
}

func metricsCount(resourceMetric *pdata.ResourceMetrics) int {
	metricsCount := 0
	ilms := resourceMetric.InstrumentationLibraryMetrics()
	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j)
		metricsCount += ilm.Metrics().Len()
	}
	return metricsCount
}

func getValidScrapes(t *testing.T, rms []*pdata.ResourceMetrics) []*pdata.ResourceMetrics {
	out := make([]*pdata.ResourceMetrics, 0)
	// rms will include failed scrapes and scrapes that received no metrics but have internal scrape metrics, filter those out
	for i := 0; i < len(rms); i++ {
		allMetrics := getMetrics(rms[i])
		if expectedScrapeMetricCount < len(allMetrics) && countScrapeMetrics(allMetrics) == expectedScrapeMetricCount {
			if isFirstFailedScrape(allMetrics) {
				continue
			}
			assertUp(t, 1, allMetrics)
			out = append(out, rms[i])
		} else {
			assertUp(t, 0, allMetrics)
		}
	}
	return out
}

func isFirstFailedScrape(metrics []*pdata.Metric) bool {
	for _, m := range metrics {
		if m.Name() == "up" {
			if m.Gauge().DataPoints().At(0).DoubleVal() == 1 { // assumed up will not have multiple datapoints
				return false
			}
		}
	}
	return true
}

func assertUp(t *testing.T, expected float64, metrics []*pdata.Metric) {
	for _, m := range metrics {
		if m.Name() == "up" {
			assert.Equal(t, expected, m.Gauge().DataPoints().At(0).DoubleVal()) // (assumed up will not have multiple datapoints)
			return
		}
	}
	t.Error("No 'up' metric found")
}

func countScrapeMetricsRM(got *pdata.ResourceMetrics) int {
	n := 0
	ilms := got.InstrumentationLibraryMetrics()
	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j)
		for i := 0; i < ilm.Metrics().Len(); i++ {
			m := ilm.Metrics().At(i)
			if isDefaultMetrics(&m) {
				n++
			}
		}
	}
	return n
}

func countScrapeMetrics(metrics []*pdata.Metric) int {
	n := 0
	for _, m := range metrics {
		if isDefaultMetrics(m) {
			n++
		}
	}
	return n
}

func isDefaultMetrics(m *pdata.Metric) bool {
	switch m.Name() {
	case "up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added":
		return true
	default:
	}
	return false
}

type metricTypeComparator func(*testing.T, *pdata.Metric)
type numberPointComparator func(*testing.T, *pdata.NumberDataPoint)
type histogramPointComparator func(*testing.T, *pdata.HistogramDataPoint)
type summaryPointComparator func(*testing.T, *pdata.SummaryDataPoint)

type dataPointExpectation struct {
	numberPointComparator    []numberPointComparator
	histogramPointComparator []histogramPointComparator
	summaryPointComparator   []summaryPointComparator
}

type testExpectation func(*testing.T, *pdata.ResourceMetrics)

func doCompare(t *testing.T, name string, want pdata.AttributeMap, got *pdata.ResourceMetrics, expectations []testExpectation) {
	t.Run(name, func(t *testing.T) {
		assert.Equal(t, expectedScrapeMetricCount, countScrapeMetricsRM(got))
		assert.Equal(t, want.Len(), got.Resource().Attributes().Len())
		for k, v := range want.AsRaw() {
			value, _ := got.Resource().Attributes().Get(k)
			assert.EqualValues(t, v, value.AsString())
		}
		for _, e := range expectations {
			e(t, got)
		}
	})
}

func assertMetricPresent(name string, metricTypeExpectations metricTypeComparator,
	dataPointExpectations []dataPointExpectation) testExpectation {
	return func(t *testing.T, rm *pdata.ResourceMetrics) {
		allMetrics := getMetrics(rm)
		for _, m := range allMetrics {
			if name != m.Name() {
				continue
			}
			metricTypeExpectations(t, m)
			for i, de := range dataPointExpectations {
				for _, npc := range de.numberPointComparator {
					switch m.DataType() {
					case pdata.MetricDataTypeGauge:
						require.Equal(t, m.Gauge().DataPoints().Len(), len(dataPointExpectations), "Expected number of data-points in Gauge metric does not match to testdata")
						dataPoint := m.Gauge().DataPoints().At(i)
						npc(t, &dataPoint)
					case pdata.MetricDataTypeSum:
						require.Equal(t, m.Sum().DataPoints().Len(), len(dataPointExpectations), "Expected number of data-points in Sum metric does not match to testdata")
						dataPoint := m.Sum().DataPoints().At(i)
						npc(t, &dataPoint)
					}
				}
				switch m.DataType() {
				case pdata.MetricDataTypeHistogram:
					for _, hpc := range de.histogramPointComparator {
						require.Equal(t, m.Histogram().DataPoints().Len(), len(dataPointExpectations), "Expected number of data-points in Histogram metric does not match to testdata")
						dataPoint := m.Histogram().DataPoints().At(i)
						hpc(t, &dataPoint)
					}
				case pdata.MetricDataTypeSummary:
					for _, spc := range de.summaryPointComparator {
						require.Equal(t, m.Summary().DataPoints().Len(), len(dataPointExpectations), "Expected number of data-points in Summary metric does not match to testdata")
						dataPoint := m.Summary().DataPoints().At(i)
						spc(t, &dataPoint)
					}
				}
			}
		}
	}
}

func assertMetricAbsent(name string) testExpectation {
	return func(t *testing.T, rm *pdata.ResourceMetrics) {
		allMetrics := getMetrics(rm)
		for _, m := range allMetrics {
			assert.NotEqual(t, name, m.Name(), "Metric is present, but was expected absent")
		}
	}
}

func compareMetricType(typ pdata.MetricDataType) metricTypeComparator {
	return func(t *testing.T, metric *pdata.Metric) {
		assert.Equal(t, typ.String(), metric.DataType().String(), "Metric type does not match")
	}
}

func compareAttributes(attributes map[string]string) numberPointComparator {
	return func(t *testing.T, numberDataPoint *pdata.NumberDataPoint) {
		req := assert.Equal(t, len(attributes), numberDataPoint.Attributes().Len(), "Attributes length do not match")
		if req {
			for k, v := range attributes {
				value, ok := numberDataPoint.Attributes().Get(k)
				if ok {
					assert.Equal(t, v, value.AsString(), "Attributes do not match")
				} else {
					assert.Fail(t, "Attributes key do not match")
				}
			}
		}
	}
}

func compareSummaryAttributes(attributes map[string]string) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint *pdata.SummaryDataPoint) {
		req := assert.Equal(t, len(attributes), summaryDataPoint.Attributes().Len(), "Summary attributes length do not match")
		if req {
			for k, v := range attributes {
				value, ok := summaryDataPoint.Attributes().Get(k)
				if ok {
					assert.Equal(t, v, value.AsString(), "Summary attributes value do not match")
				} else {
					assert.Fail(t, "Summary attributes key do not match")
				}
			}
		}
	}
}

func compareHistogramAttributes(attributes map[string]string) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint *pdata.HistogramDataPoint) {
		req := assert.Equal(t, len(attributes), histogramDataPoint.Attributes().Len(), "Histogram attributes length do not match")
		if req {
			for k, v := range attributes {
				value, ok := histogramDataPoint.Attributes().Get(k)
				if ok {
					assert.Equal(t, v, value.AsString(), "Histogram attributes value do not match")
				} else {
					assert.Fail(t, "Histogram attributes key do not match")
				}
			}
		}
	}
}

func compareStartTimestamp(startTimeStamp pdata.Timestamp) numberPointComparator {
	return func(t *testing.T, numberDataPoint *pdata.NumberDataPoint) {
		assert.Equal(t, startTimeStamp.String(), numberDataPoint.StartTimestamp().String(), "Start-Timestamp does not match")
	}
}

func compareTimestamp(timeStamp pdata.Timestamp) numberPointComparator {
	return func(t *testing.T, numberDataPoint *pdata.NumberDataPoint) {
		assert.Equal(t, timeStamp.String(), numberDataPoint.Timestamp().String(), "Timestamp does not match")
	}
}

func compareHistogramTimestamp(timeStamp pdata.Timestamp) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint *pdata.HistogramDataPoint) {
		assert.Equal(t, timeStamp.String(), histogramDataPoint.Timestamp().String(), "Histogram Timestamp does not match")
	}
}

func compareHistogramStartTimestamp(timeStamp pdata.Timestamp) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint *pdata.HistogramDataPoint) {
		assert.Equal(t, timeStamp.String(), histogramDataPoint.StartTimestamp().String(), "Histogram Start-Timestamp does not match")
	}
}

func compareSummaryTimestamp(timeStamp pdata.Timestamp) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint *pdata.SummaryDataPoint) {
		assert.Equal(t, timeStamp.String(), summaryDataPoint.Timestamp().String(), "Summary Timestamp does not match")
	}
}

func compareSummaryStartTimestamp(timeStamp pdata.Timestamp) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint *pdata.SummaryDataPoint) {
		assert.Equal(t, timeStamp.String(), summaryDataPoint.StartTimestamp().String(), "Summary Start-Timestamp does not match")
	}
}

func compareDoubleValue(doubleVal float64) numberPointComparator {
	return func(t *testing.T, numberDataPoint *pdata.NumberDataPoint) {
		assert.Equal(t, doubleVal, numberDataPoint.DoubleVal(), "Metric double value does not match")
	}
}

func compareHistogram(count uint64, sum float64, buckets []uint64) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint *pdata.HistogramDataPoint) {
		assert.Equal(t, count, histogramDataPoint.Count(), "Histogram count value does not match")
		assert.Equal(t, sum, histogramDataPoint.Sum(), "Histogram sum value does not match")
		assert.Equal(t, buckets, histogramDataPoint.BucketCounts(), "Histogram bucket count values do not match")
	}
}

func compareSummary(count uint64, sum float64, quantiles [][]float64) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint *pdata.SummaryDataPoint) {
		assert.Equal(t, count, summaryDataPoint.Count(), "Summary count value does not match")
		assert.Equal(t, sum, summaryDataPoint.Sum(), "Summary sum value does not match")
		req := assert.Equal(t, len(quantiles), summaryDataPoint.QuantileValues().Len())
		if req {
			for i := 0; i < summaryDataPoint.QuantileValues().Len(); i++ {
				assert.Equal(t, quantiles[i][0], summaryDataPoint.QuantileValues().At(i).Quantile(), "Summary quantile do not match")
				assert.Equal(t, quantiles[i][1], summaryDataPoint.QuantileValues().At(i).Value(), "Summary quantile values do not match")
			}
		}
	}
}

func testComponent(t *testing.T, targets []*testData, useStartTimeMetric bool, startTimeMetricRegex string, useOpenMetrics bool) {
	// 1. setup mock server
	mp, cfg, err := setupMockPrometheus(useOpenMetrics, targets...)
	require.Nilf(t, err, "Failed to create Prometheus config: %v", err)
	defer mp.Close()

	cms := new(consumertest.MetricsSink)
	rcvr := newPrometheusReceiver(componenttest.NewNopReceiverCreateSettings(), &Config{
		ReceiverSettings:     config.NewReceiverSettings(config.NewComponentID(typeStr)),
		PrometheusConfig:     cfg,
		UseStartTimeMetric:   useStartTimeMetric,
		StartTimeMetricRegex: startTimeMetricRegex}, cms)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()), "Failed to invoke Start: %v", err)
	t.Cleanup(func() {
		// verify state after shutdown is called
		assert.Lenf(t, flattenTargets(rcvr.scrapeManager.TargetsAll()), len(targets), "expected %v targets to be running", len(targets))
		require.NoError(t, rcvr.Shutdown(context.Background()))
		assert.Len(t, flattenTargets(rcvr.scrapeManager.TargetsAll()), 0, "expected scrape manager to have no targets")
	})

	// wait for all provided data to be scraped
	mp.wg.Wait()
	metrics := cms.AllMetrics()

	// split and store results by target name
	pResults := splitMetricsByTarget(metrics)
	lres, lep := len(pResults), len(mp.endpoints)
	assert.Equalf(t, lep, lres, "want %d targets, but got %v\n", lep, lres)

	// loop to validate outputs for each targets
	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			validScrapes := pResults[target.name]
			if !useOpenMetrics {
				validScrapes = getValidScrapes(t, pResults[target.name])
			}
			target.validateFunc(t, target, validScrapes)
		})
	}
}

// starts prometheus receiver with custom config, retrieves metrics from MetricsSink
func testComponentCustomConfig(t *testing.T, targets []*testData, mp *mockPrometheus, cfg *promcfg.Config) {
	ctx := context.Background()
	defer mp.Close()

	cms := new(consumertest.MetricsSink)
	receiver := newPrometheusReceiver(componenttest.NewNopReceiverCreateSettings(), &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		PrometheusConfig: cfg}, cms)

	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))

	// verify state after shutdown is called
	t.Cleanup(func() { require.NoError(t, receiver.Shutdown(ctx)) })

	// wait for all provided data to be scraped
	mp.wg.Wait()
	metrics := cms.AllMetrics()

	// split and store results by target name
	pResults := splitMetricsByTarget(metrics)
	lres, lep := len(pResults), len(mp.endpoints)
	assert.Equalf(t, lep, lres, "want %d targets, but got %v\n", lep, lres)

	// loop to validate outputs for each targets
	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			validScrapes := getValidScrapes(t, pResults[target.name])
			target.validateFunc(t, target, validScrapes)
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

func splitMetricsByTarget(metrics []pdata.Metrics) map[string][]*pdata.ResourceMetrics {
	pResults := make(map[string][]*pdata.ResourceMetrics)
	for _, md := range metrics {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			name, _ := rms.At(i).Resource().Attributes().Get("service.name")
			pResult, ok := pResults[name.AsString()]
			if !ok {
				pResult = make([]*pdata.ResourceMetrics, 0)
			}
			rm := rms.At(i)
			pResults[name.AsString()] = append(pResult, &rm)
		}
	}
	return pResults
}
