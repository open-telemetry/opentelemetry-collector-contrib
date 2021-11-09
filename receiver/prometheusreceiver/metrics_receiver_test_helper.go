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
	"fmt"
	"go.opentelemetry.io/collector/model/pdata"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	gokitlog "github.com/go-kit/log"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

type mockPrometheusResponse struct {
	code int
	data string
}

type mockPrometheus struct {
	mu          sync.Mutex // mu protects the fields below.
	endpoints   map[string][]mockPrometheusResponse
	accessIndex map[string]*int32
	wg          *sync.WaitGroup
	srv         *httptest.Server
}

func newMockPrometheus(endpoints map[string][]mockPrometheusResponse) *mockPrometheus {
	accessIndex := make(map[string]*int32)
	wg := &sync.WaitGroup{}
	wg.Add(len(endpoints))
	for k := range endpoints {
		v := int32(0)
		accessIndex[k] = &v
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
	srvPlaceHolder            = "__SERVER_ADDRESS__"
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
func setupMockPrometheus(tds ...*testData) (*mockPrometheus, *promcfg.Config, error) {
	jobs := make([]map[string]interface{}, 0, len(tds))
	endpoints := make(map[string][]mockPrometheusResponse)
	for _, t := range tds {
		metricPath := fmt.Sprintf("/%s/metrics", t.name)
		endpoints[metricPath] = t.pages
		job := make(map[string]interface{})
		job["job_name"] = t.name
		job["metrics_path"] = metricPath
		job["scrape_interval"] = "1s"
		job["static_configs"] = []map[string]interface{}{{"targets": []string{srvPlaceHolder}}}
		jobs = append(jobs, job)
	}

	if len(jobs) != len(tds) {
		log.Fatal("len(jobs) != len(targets), make sure job names are unique")
	}
	config := make(map[string]interface{})
	config["scrape_configs"] = jobs

	mp := newMockPrometheus(endpoints)
	cfg, err := yaml.Marshal(&config)
	if err != nil {
		return mp, nil, err
	}
	u, _ := url.Parse(mp.srv.URL)
	host, port, _ := net.SplitHostPort(u.Host)

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

	cfgStr := strings.ReplaceAll(string(cfg), srvPlaceHolder, u.Host)
	pCfg, err := promcfg.Load(cfgStr, false, gokitlog.NewNopLogger())
	return mp, pCfg, err
}

func verifyNumScrapeResults(t *testing.T, td *testData, resourceMetrics []*pdata.ResourceMetrics) {
	want := 0
	for _, p := range td.pages {
		if p.code == 200 {
			want++
		}
	}
	if l := len(resourceMetrics); l != want {
		t.Fatalf("want %d, but got %d\n", want, l)
	}
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
			switch ilm.Metrics().At(i).Name() {
			case "up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added":
				n++
			default:
			}
		}
	}
	return n
}

func countScrapeMetrics(metrics []*pdata.Metric) int {
	n := 0
	for _, m := range metrics {
		switch m.Name() {
		case "up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added":
			n++
		default:
		}
	}
	return n
}

type metricTypeComparator func(*testing.T, *pdata.Metric) bool
type numberPointComparator func(*testing.T, *pdata.NumberDataPoint) bool
type histogramPointComparator func(*testing.T, *pdata.HistogramDataPoint) bool
type summaryPointComparator func(*testing.T, *pdata.SummaryDataPoint) bool

type dataPointExpectation struct {
	numberPointComparator    []numberPointComparator
	histogramPointComparator []histogramPointComparator
	summaryPointComparator   []summaryPointComparator
}

type testExpectation func(*testing.T, *pdata.ResourceMetrics) bool

func doCompare(name string, t *testing.T, want pdata.AttributeMap, got *pdata.ResourceMetrics, expectations []testExpectation) {
	t.Run(name, func(t *testing.T) {
		assert.Equal(t, expectedScrapeMetricCount, countScrapeMetricsRM(got))
		assert.Equal(t, want.Len(), got.Resource().Attributes().Len())
		for k, v := range want.AsRaw() {
			value, _ := got.Resource().Attributes().Get(k)
			assert.EqualValues(t, v, value.AsString())
		}
		for _, e := range expectations {
			assert.True(t, e(t, got))
		}
	})
}

func assertMetricPresent(name string, metricTypeExpectations metricTypeComparator,
	dataPointExpectations []dataPointExpectation) testExpectation {
	return func(t *testing.T, rm *pdata.ResourceMetrics) bool {
		allMetrics := getMetrics(rm)
		for _, m := range allMetrics {
			if name != m.Name() {
				continue
			}
			if !metricTypeExpectations(t, m) {
				return false
			}
			for i, de := range dataPointExpectations {

				for _, npc := range de.numberPointComparator {
					switch m.DataType() {
					case pdata.MetricDataTypeGauge:
						dataPoint := m.Gauge().DataPoints().At(i)
						if !npc(t, &dataPoint) {
							return false
						}
					case pdata.MetricDataTypeSum:
						dataPoint := m.Sum().DataPoints().At(i)
						if !npc(t, &dataPoint) {
							return false
						}
					}
				}

				switch m.DataType() {
				case pdata.MetricDataTypeHistogram:
					for _, hpc := range de.histogramPointComparator {
						dataPoint := m.Histogram().DataPoints().At(i)
						if !hpc(t, &dataPoint) {
							return false
						}
					}
				case pdata.MetricDataTypeSummary:
					for _, spc := range de.summaryPointComparator {
						dataPoint := m.Summary().DataPoints().At(i)
						if !spc(t, &dataPoint) {
							return false
						}
					}
				}
			}
			return true
		}
		assert.Failf(t, "Unable to match metric expectation", name)
		return false
	}
}

func assertMetricAbsent(name string) testExpectation {
	return func(t *testing.T, rm *pdata.ResourceMetrics) bool {
		allMetrics := getMetrics(rm)
		for _, m := range allMetrics {
			if !assert.NotEqual(t, name, m.Name()) {
				return false
			}
		}
		return true
	}
}

func compareMetricType(typ pdata.MetricDataType) metricTypeComparator {
	return func(t *testing.T, metric *pdata.Metric) bool {
		return assert.Equal(t, typ, metric.DataType())
	}
}

func compareAttributes(attributes map[string]string) numberPointComparator {
	return func(t *testing.T, numberDataPoint *pdata.NumberDataPoint) bool {
		if !assert.Equal(t, len(attributes), numberDataPoint.Attributes().Len()) {
			return false
		}
		for k, v := range attributes {
			value, ok := numberDataPoint.Attributes().Get(k)
			if !ok || !assert.Equal(t, v, value.AsString()) {
				return false
			}
		}
		return true
	}
}

func compareHistogramAttributes(attributes map[string]string) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint *pdata.HistogramDataPoint) bool {
		if !assert.Equal(t, len(attributes), histogramDataPoint.Attributes().Len()) {
			return false
		}
		for k, v := range attributes {
			value, ok := histogramDataPoint.Attributes().Get(k)
			if !ok || !assert.Equal(t, v, value.AsString()) {
				return false
			}
		}
		return true
	}
}

func compareSummaryAttributes(attributes map[string]string) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint *pdata.SummaryDataPoint) bool {
		if !assert.Equal(t, len(attributes), summaryDataPoint.Attributes().Len()) {
			return false
		}
		for k, v := range attributes {
			value, ok := summaryDataPoint.Attributes().Get(k)
			if !ok || !assert.Equal(t, v, value.AsString()) {
				return false
			}
		}
		return true
	}
}

func compareStartTimestamp(startTimeStamp pdata.Timestamp) numberPointComparator {
	return func(t *testing.T, numberDataPoint *pdata.NumberDataPoint) bool {
		return assert.Equal(t, startTimeStamp, numberDataPoint.StartTimestamp())
	}
}

func compareTimestamp(timeStamp pdata.Timestamp) numberPointComparator {
	return func(t *testing.T, numberDataPoint *pdata.NumberDataPoint) bool {
		return assert.Equal(t, timeStamp, numberDataPoint.Timestamp())
	}
}

func compareHistogramTimestamp(timeStamp pdata.Timestamp) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint *pdata.HistogramDataPoint) bool {
		return assert.Equal(t, timeStamp, histogramDataPoint.Timestamp())
	}
}

func compareHistogramStartTimestamp(timeStamp pdata.Timestamp) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint *pdata.HistogramDataPoint) bool {
		return assert.Equal(t, timeStamp, histogramDataPoint.StartTimestamp())
	}
}

func compareSummaryTimestamp(timeStamp pdata.Timestamp) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint *pdata.SummaryDataPoint) bool {
		return assert.Equal(t, timeStamp, summaryDataPoint.Timestamp())
	}
}

func compareSummaryStartTimestamp(timeStamp pdata.Timestamp) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint *pdata.SummaryDataPoint) bool {
		return assert.Equal(t, timeStamp, summaryDataPoint.StartTimestamp())
	}
}

func compareDoubleValue(doubleVal float64) numberPointComparator {
	return func(t *testing.T, numberDataPoint *pdata.NumberDataPoint) bool {
		return assert.Equal(t, doubleVal, numberDataPoint.DoubleVal())
	}
}

func compareHistogram(count uint64, sum float64, buckets []uint64) histogramPointComparator {
	return func(t *testing.T, histogramDataPoint *pdata.HistogramDataPoint) bool {
		ret := assert.Equal(t, count, histogramDataPoint.Count())
		ret = ret && assert.Equal(t, sum, histogramDataPoint.Sum())
		if ret {
			ret = ret && assert.Equal(t, buckets, histogramDataPoint.BucketCounts())
		}
		return ret
	}
}

func compareSummary(count uint64, sum float64, quantiles [][]float64) summaryPointComparator {
	return func(t *testing.T, summaryDataPoint *pdata.SummaryDataPoint) bool {
		ret := assert.Equal(t, count, summaryDataPoint.Count())
		ret = ret && assert.Equal(t, sum, summaryDataPoint.Sum())

		if ret {
			assert.Equal(t, len(quantiles), summaryDataPoint.QuantileValues().Len())
			for i := 0; i < summaryDataPoint.QuantileValues().Len(); i++ {
				assert.Equal(t, quantiles[i][0], summaryDataPoint.QuantileValues().At(i).Quantile())
				assert.Equal(t, quantiles[i][1], summaryDataPoint.QuantileValues().At(i).Value())
			}
		}
		return ret
	}
}
