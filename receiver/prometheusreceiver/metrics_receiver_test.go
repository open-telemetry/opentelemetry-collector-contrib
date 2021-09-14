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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gokitlog "github.com/go-kit/kit/log"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v2"
)

var logger = zap.NewNop()

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
	resource     pdata.Resource
	validateFunc func(t *testing.T, td *testData, rmsL []pdata.ResourceMetrics)
}

// Given that pdata doesn't allow us to trivially construct pdata.Metrics,
// we can extract pdata.ResourceMetrics from each value.
func resourceMetricsFromMetrics(metricsL []pdata.Metrics) (rmsL []pdata.ResourceMetrics) {
	for _, metrics := range metricsL {
		irmsL := metrics.ResourceMetrics()
		for i := 0; i < irmsL.Len(); i++ {
			rmsL = append(rmsL, irmsL.At(i))
		}
	}
	return rmsL
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

	// update node value (will use for validation)
	for _, t := range tds {
		rsc := pdata.NewResource()
		attrs := rsc.Attributes()
		attrs.InsertString("instance", u.Host)
		attrs.InsertString("scheme", "http")
		attrs.InsertString("port", port)
		attrs.InsertString("job", t.name)
		attrs.InsertString("host.name", host)
		attrs.InsertString("service.name", t.name)
		t.resource = rsc
	}

	cfgStr := strings.ReplaceAll(string(cfg), srvPlaceHolder, u.Host)
	pCfg, err := promcfg.Load(cfgStr, false, gokitlog.NewNopLogger())
	return mp, pCfg, err
}

func verifyNumScrapeResults(t *testing.T, td *testData, metricsL []pdata.ResourceMetrics) {
	want := 0
	for _, p := range td.pages {
		if p.code == 200 {
			want++
		}
	}
	if l := len(metricsL); l != want {
		t.Errorf("want %d, but got %d\n", want, l)
	}
}

func doCompare(t *testing.T, name string, want, got pdata.ResourceMetrics) {
	// Ensure that the resource attributes can be deterministically compared.
	got.Resource().Attributes().Sort()
	want.Resource().Attributes().Sort()

	t.Run(name, func(t *testing.T) {
		assert.Equal(t, want.Resource(), got.Resource(), "Resource mismatch")
		assert.Equal(t, want.SchemaUrl(), got.SchemaUrl(), "Resource mismatch")
		assert.Equal(t, want.InstrumentationLibraryMetrics(), got.InstrumentationLibraryMetrics())
	})
}

func getValidScrapes(t *testing.T, metricsL []pdata.ResourceMetrics) (out []pdata.ResourceMetrics) {
	return metricsL
	panic("FIX ME")
	/*
		for _, metrics := range metricsL {
			// mds will include scrapes that received no metrics but have internal scrape metrics, filter those out
			if expectedScrapeMetricCount < len(md.Metrics) && countScrapeMetrics(md) == expectedScrapeMetricCount {
				assertUp(t, 1, md)
				out = append(out, md)
			} else {
				assertUp(t, 0, md)
			}
		}
		return out
	*/
}

func assertUp(t *testing.T, expected float64, metrics pdata.ResourceMetrics) {
	panic("FIX ME")
	/*
		for _, m := range md.Metrics {
			if m.GetMetricDescriptor().Name == "up" {
				assert.Equal(t, expected, m.Timeseries[0].Points[0].GetDoubleValue())
				return
			}
		}
		t.Error("No 'up' metric found")
	*/
}

func countScrapeMetrics(in pdata.ResourceMetrics) int {
	panic("FIX ME")
	/*
		n := 0
		for _, m := range in.Metrics {
			switch m.MetricDescriptor.Name {
			case "up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added":
				n++
			default:
			}
		}
		return n
	*/
}

// Test data and validation functions for EndToEnd test
// Make sure every page has a gauge, we are relying on it to figure out the start time if needed

// target1 has one gauge, two counts of a same family, one histogram and one summary. We are expecting the both
// successful scrapes will produce all metrics using the first scrape's timestamp as start time.
var target1Page1 = `
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

var target1Page2 = `
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

func dataPointCount(rms pdata.ResourceMetrics) int {
	return rms.InstrumentationLibraryMetrics().At(0).Metrics().Len()
}

func extractFirstTimestamp(rms pdata.ResourceMetrics) (startTimestamp, timestamp pdata.Timestamp) {
	startTs, ts, _ := extractFirstTimestampDataType(rms)
	return startTs, ts
}

func extractFirstTimestampDataType(rms pdata.ResourceMetrics) (startTimestamp, timestamp pdata.Timestamp, _ pdata.MetricDataType) {
	metric := rms.InstrumentationLibraryMetrics().At(0).Metrics().At(0)
	switch dataType := metric.DataType(); dataType {
	case pdata.MetricDataTypeGauge:
		point0 := metric.Gauge().DataPoints().At(0)
		return point0.StartTimestamp(), point0.Timestamp(), dataType
	case pdata.MetricDataTypeSum:
		point0 := metric.Sum().DataPoints().At(0)
		return point0.StartTimestamp(), point0.Timestamp(), dataType
	case pdata.MetricDataTypeHistogram:
		point0 := metric.Histogram().DataPoints().At(0)
		return point0.StartTimestamp(), point0.Timestamp(), dataType
	case pdata.MetricDataTypeSummary:
		point0 := metric.Summary().DataPoints().At(0)
		return point0.StartTimestamp(), point0.Timestamp(), dataType
	default:
		panic("Unhandled " + metric.DataType().String())
	}
}

const ts1 = pdata.Timestamp(1e9)
const ts2 = pdata.Timestamp(2e9)

func verifyTarget1(t *testing.T, td *testData, metricsL []pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, metricsL)
	if len(metricsL) < 1 {
		t.Fatal("At least one metric request should be present")
	}

	// Extract the first startTimestamp.
	got1 := metricsL[0]

	// got 1 has 4 metrics + 5 internal scraper metrics
	if g, w := dataPointCount(got1), 9; g != w {
		t.Fatalf("got %d, want %d", g, w)
	}
	startTs1, ts1 := extractFirstTimestamp(got1)

	want1 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs1, makeDoublePoint(ts1, 19.0)),
		makeSumMetric("http_requests_total", startTs1,
			makeDoublePoint(ts1, 100.0, kv{"code", "200"}, kv{"method", "post"}),
			makeDoublePoint(ts1, 3, kv{"code", "400"}, kv{"method", "post"}),
		),
		makeCumulativeDistMetric("http_request_duration_seconds", startTs1,
			makeDistPoint(ts1, 2500, 5000, []uint64{1000, 500, 500, 500}, []float64{0.05, 0.5, 1}),
		),
		makeSummaryMetric("rpc_duration_seconds", startTs1,
			makeSummaryPoint(ts1, 1000, 5000, map[float64]float64{1: 1, 90: 5, 99: 8}),
		),
		makeGaugeMetric("up", startTs1, makeDoublePoint(ts1, 1.0)),
		makeGaugeMetric("scrape_duration_seconds", startTs1, makeDoublePoint(ts1, 0.001662471)),
		makeGaugeMetric("scrape_samples_scraped", startTs1, makeDoublePoint(ts1, 18.0)),
		makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs1, makeDoublePoint(ts1, 18.0)),
		makeGaugeMetric("scrape_series_added", startTs1, makeDoublePoint(ts1, 18.0)),
	)

	doCompare(t, "scrape1", want1, got1)

	got2 := metricsL[1]
	startTs2, ts2 := extractFirstTimestamp(got2)
	// Verify the 2nd data.
	want2 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs2, makeDoublePoint(ts2, 18.0)),
		makeSumMetric("http_requests_total", startTs2,
			makeDoublePoint(ts2, 199, kv{"code", "200"}, kv{"method", "post"}),
			makeDoublePoint(ts2, 12, kv{"code", "400"}, kv{"method", "post"}),
		),
		makeCumulativeDistMetric("http_requests_duration_seconds", startTs2,
			makeDistPoint(ts2, 2600, 5050, []uint64{1100, 500, 500, 500}, []float64{0.05, 0.5, 1}),
		),
		makeSummaryMetric("rpc_duration_seconds", startTs2,
			makeSummaryPoint(ts2, 1000, 5000, map[float64]float64{1: 1, 90: 6, 99: 8}),
		),
		makeGaugeMetric("up", startTs1, makeDoublePoint(ts1, 1.0)),
		makeGaugeMetric("scrape_duration_seconds", startTs1, makeDoublePoint(ts1, 0.001662471)),
		makeGaugeMetric("scrape_samples_scraped", startTs1, makeDoublePoint(ts1, 18.0)),
		makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs1, makeDoublePoint(ts1, 18.0)),
		makeGaugeMetric("scrape_series_added", startTs1, makeDoublePoint(ts1, 18.0)),
	)

	doCompare(t, "scrape2", want2, got2)
}

type kv struct {
	key, value string
}

func makeDoublePoint(ts pdata.Timestamp, value float64, kvp ...kv) pdata.NumberDataPoint {
	ndp := pdata.NewNumberDataPoint()
	ndp.SetTimestamp(ts)
	ndp.SetDoubleVal(value)

	attrs := ndp.Attributes()
	for _, kv := range kvp {
		attrs.InsertString(kv.key, kv.value)
	}
	return ndp
}

func makeGaugeMetric(name string, startTs pdata.Timestamp, points ...pdata.NumberDataPoint) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeGauge)

	destPointL := metric.Gauge().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		destPoint.SetStartTimestamp(startTs)
	}
	return metric
}

func makeSummaryPoint(ts pdata.Timestamp, count uint64, sum float64, qvm map[float64]float64, kvp ...kv) pdata.SummaryDataPoint {
	sdp := pdata.NewSummaryDataPoint()
	sdp.SetTimestamp(ts)
	sdp.SetCount(count)
	sdp.SetSum(sum)
	qvL := sdp.QuantileValues()
	for quantile, value := range qvm {
		qvi := qvL.AppendEmpty()
		qvi.SetQuantile(quantile)
		qvi.SetValue(value)
	}

	attrs := sdp.Attributes()
	for _, kv := range kvp {
		attrs.InsertString(kv.key, kv.value)
	}
	return sdp
}

func makeSummaryMetric(name string, startTs pdata.Timestamp, points ...pdata.SummaryDataPoint) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSummary)

	destPointL := metric.Summary().DataPoints()
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		destPoint.SetStartTimestamp(startTs)
	}
	return metric
}

func makeSumMetric(name string, startTs pdata.Timestamp, points ...pdata.NumberDataPoint) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeSum)
	sum := metric.Sum()
	sum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	destPointL := sum.DataPoints()

	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		destPoint.SetStartTimestamp(startTs)
	}
	return metric
}

func makeDistPoint(ts pdata.Timestamp, count uint64, sum float64, counts []uint64, bounds []float64, kvp ...kv) pdata.HistogramDataPoint {
	hdp := pdata.NewHistogramDataPoint()
	hdp.SetExplicitBounds(bounds)
	hdp.SetBucketCounts(counts)
	hdp.SetTimestamp(ts)
	hdp.SetCount(count)
	hdp.SetSum(sum)

	attrs := hdp.Attributes()
	for _, kv := range kvp {
		attrs.InsertString(kv.key, kv.value)
	}
	return hdp
}

func makeGaugeDistMetric(name string, startTs pdata.Timestamp, points ...pdata.HistogramDataPoint) pdata.Metric {
	hMetric := makeCumulativeDistMetric(name, startTs, points...)
	hMetric.Histogram().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	return hMetric
}

func makeCumulativeDistMetric(name string, startTs pdata.Timestamp, points ...pdata.HistogramDataPoint) pdata.Metric {
	metric := pdata.NewMetric()
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeHistogram)
	histogram := metric.Histogram()
	histogram.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)

	destPointL := histogram.DataPoints()
	// By default the AggregationTemporality is Cumulative until it'll be changed by the caller.
	for _, point := range points {
		destPoint := destPointL.AppendEmpty()
		point.CopyTo(destPoint)
		destPoint.SetStartTimestamp(startTs)
	}
	return metric
}

// target2 is going to have 5 pages, and there's a newly added item on the 2nd page.
// with the 4th page, we are simulating a reset (values smaller than previous), start times should be from
// this run for the 4th and 5th scrapes.
var target2Page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 10
http_requests_total{method="post",code="400"} 50
`

var target2Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="400"} 60
http_requests_total{method="post",code="500"} 3
`

var target2Page3 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="400"} 60
http_requests_total{method="post",code="500"} 5
`

var target2Page4 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 49
http_requests_total{method="post",code="400"} 59
http_requests_total{method="post",code="500"} 3
`

var target2Page5 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 50
http_requests_total{method="post",code="400"} 59
http_requests_total{method="post",code="500"} 5
`

func verifyTarget2(t *testing.T, td *testData, metricsL []pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, metricsL)
	if len(metricsL) < 1 {
		t.Fatal("At least one metric request should be present")
	}

	// Extract the first startTimestamp.
	got1 := metricsL[0]

	// got has 2 metrics + 5 internal scraper metrics
	if g, w := dataPointCount(got1), 7; g != w {
		t.Fatalf("got %d, want %d", g, w)
	}
	startTs1, ts1 := extractFirstTimestamp(got1)

	want1 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs1, makeDoublePoint(ts1, 18.0)),
		makeSumMetric("http_requests_total", startTs1,
			makeDoublePoint(ts1, 10, kv{"code", "200"}, kv{"method", "post"}),
			makeDoublePoint(ts1, 50, kv{"code", "400"}, kv{"method", "post"}),
		),
		makeGaugeMetric("up", startTs1, makeDoublePoint(ts1, 1.0)),
		makeGaugeMetric("scrape_duration_seconds", startTs1, makeDoublePoint(ts1, 0.001662471)),
		makeGaugeMetric("scrape_samples_scraped", startTs1, makeDoublePoint(ts1, 14.0)),
		makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs1, makeDoublePoint(ts1, 14.0)),
		makeGaugeMetric("scrape_series_added", startTs1, makeDoublePoint(ts1, 14.0)),
	)
	doCompare(t, "scrape1", want1, got1)

	// Verify the 2nd data.
	got2 := metricsL[1]
	startTs2, ts2 := extractFirstTimestamp(got2)
	want2 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs2, makeDoublePoint(ts2, 16.0)),
		makeSumMetric("http_requests_total", startTs2,
			makeDoublePoint(ts2, 50, kv{"code", "200"}, kv{"method", "post"}),
			makeDoublePoint(ts2, 60, kv{"code", "400"}, kv{"method", "post"}),
			makeDoublePoint(ts2, 3, kv{"code", "500"}, kv{"method", "post"}),
		),
		makeGaugeMetric("up", startTs1, makeDoublePoint(ts1, 1.0)),
		makeGaugeMetric("scrape_duration_seconds", startTs1, makeDoublePoint(ts1, 0.001662471)),
		makeGaugeMetric("scrape_samples_scraped", startTs1, makeDoublePoint(ts1, 14.0)),
		makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs1, makeDoublePoint(ts1, 14.0)),
		makeGaugeMetric("scrape_series_added", startTs1, makeDoublePoint(ts1, 14.0)),
	)

	doCompare(t, "scrape2", want2, got2)

	// Verify the 3rd, with the new coce=500 counter which first appeared on the 2nd run.
	got3 := metricsL[2]
	startTs3, ts3 := extractFirstTimestamp(got3)
	want3 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs3, makeDoublePoint(ts3, 16.0)),
		makeSumMetric("http_requests_total", startTs3,
			makeDoublePoint(ts1, 50, kv{"code", "200"}, kv{"method", "post"}),
			makeDoublePoint(ts3, 60, kv{"code", "400"}, kv{"method", "post"}),
			makeDoublePoint(ts3, 5, kv{"code", "500"}, kv{"method", "post"}),
		),
		makeGaugeMetric("up", startTs1, makeDoublePoint(ts1, 1.0)),
		makeGaugeMetric("scrape_duration_seconds", startTs1, makeDoublePoint(ts1, 0.001662471)),
		makeGaugeMetric("scrape_samples_scraped", startTs1, makeDoublePoint(ts1, 14.0)),
		makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs1, makeDoublePoint(ts1, 14.0)),
		makeGaugeMetric("scrape_series_added", startTs1, makeDoublePoint(ts1, 14.0)),
	)
	doCompare(t, "scrape3", want3, got3)

	got4 := metricsL[3]
	startTs4, ts4 := extractFirstTimestamp(got4)
	want4 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs4, makeDoublePoint(ts4, 16.0)),
		makeSumMetric("http_requests_total", startTs3,
			makeDoublePoint(ts4, 50, kv{"code", "200"}, kv{"method", "post"}),
			makeDoublePoint(ts4, 60, kv{"code", "400"}, kv{"method", "post"}),
			makeDoublePoint(ts4, 5, kv{"code", "500"}, kv{"method", "post"}),
		),
		makeGaugeMetric("up", startTs4, makeDoublePoint(ts4, 1.0)),
		makeGaugeMetric("scrape_duration_seconds", startTs4, makeDoublePoint(ts4, 0.001662471)),
		makeGaugeMetric("scrape_samples_scraped", startTs4, makeDoublePoint(ts4, 14.0)),
		makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs4, makeDoublePoint(ts4, 14.0)),
		makeGaugeMetric("scrape_series_added", startTs4, makeDoublePoint(ts4, 14.0)),
	)
	doCompare(t, "scrape4", want4, got4)

	got5 := metricsL[4]
	startTs5, ts5 := extractFirstTimestamp(got5)
	want5 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs5, makeDoublePoint(ts5, 16.0)),
		makeSumMetric("http_requests_total", startTs5,
			makeDoublePoint(ts4, 49, kv{"code", "200"}, kv{"method", "post"}),
			makeDoublePoint(ts4, 60, kv{"code", "400"}, kv{"method", "post"}),
			makeDoublePoint(ts3, 3, kv{"code", "500"}, kv{"method", "post"}),
		),
		makeGaugeMetric("up", startTs1, makeDoublePoint(ts1, 1.0)),
		makeGaugeMetric("scrape_duration_seconds", startTs1, makeDoublePoint(ts1, 0.001662471)),
		makeGaugeMetric("scrape_samples_scraped", startTs1, makeDoublePoint(ts1, 4.0)),
		makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs1, makeDoublePoint(ts1, 4.0)),
		makeGaugeMetric("scrape_series_added", startTs1, makeDoublePoint(ts1, 4.0)),
	)
	doCompare(t, "scrape5", want5, got5)
}

// target3 for complicated data types, including summaries and histograms. one of the summary and histogram have only
// sum/count, for the summary it's valid, however the histogram one is not, but it shall not cause the scrape to fail
var target3Page1 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 18

# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.2"} 10000
http_request_duration_seconds_bucket{le="0.5"} 11000
http_request_duration_seconds_bucket{le="1"} 12001
http_request_duration_seconds_bucket{le="+Inf"} 13003
http_request_duration_seconds_sum 50000
http_request_duration_seconds_count 13003

# A corrupted histogram with only sum and count
# HELP corrupted_hist A corrupted_hist.
# TYPE corrupted_hist histogram
corrupted_hist_sum 100
corrupted_hist_count 10

# Finally a summary, which has a complex representation, too:
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{foo="bar" quantile="0.01"} 31
rpc_duration_seconds{foo="bar" quantile="0.05"} 35
rpc_duration_seconds{foo="bar" quantile="0.5"} 47
rpc_duration_seconds{foo="bar" quantile="0.9"} 70
rpc_duration_seconds{foo="bar" quantile="0.99"} 76
rpc_duration_seconds_sum{foo="bar"} 8000
rpc_duration_seconds_count{foo="bar"} 900
rpc_duration_seconds_sum{foo="no_quantile"} 100
rpc_duration_seconds_count{foo="no_quantile"} 50
`

var target3Page2 = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 16

# A histogram, which has a pretty complex representation in the text format:
# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.2"} 11000
http_request_duration_seconds_bucket{le="0.5"} 12000
http_request_duration_seconds_bucket{le="1"} 13001
http_request_duration_seconds_bucket{le="+Inf"} 14003
http_request_duration_seconds_sum 50100
http_request_duration_seconds_count 14003

# A corrupted histogram with only sum and count
# HELP corrupted_hist A corrupted_hist.
# TYPE corrupted_hist histogram
corrupted_hist_sum 101
corrupted_hist_count 15

# Finally a summary, which has a complex representation, too:
# HELP rpc_duration_seconds A summary of the RPC duration in seconds.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds{foo="bar" quantile="0.01"} 32
rpc_duration_seconds{foo="bar" quantile="0.05"} 35
rpc_duration_seconds{foo="bar" quantile="0.5"} 47
rpc_duration_seconds{foo="bar" quantile="0.9"} 70
rpc_duration_seconds{foo="bar" quantile="0.99"} 77
rpc_duration_seconds_sum{foo="bar"} 8100
rpc_duration_seconds_count{foo="bar"} 950
rpc_duration_seconds_sum{foo="no_quantile"} 101
rpc_duration_seconds_count{foo="no_quantile"} 55
`

func verifyTarget3(t *testing.T, td *testData, metricsL []pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, metricsL)
	if len(metricsL) < 1 {
		t.Fatal("At least one metric request should be present")
	}

	// Extract the first startTimestamp.
	got1 := metricsL[0]

	// got has 8 metrics + 5 internal scraper metrics
	if g, w := dataPointCount(got1), 8; g != w {
		t.Fatalf("got %d, want %d", g, w)
	}
	startTs1, ts1 := extractFirstTimestamp(got1)

	want1 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs1, makeDoublePoint(ts1, 18.0)),
		makeCumulativeDistMetric("http_request_duration_seconds", startTs1,
			makeDistPoint(ts1, 13003, 50000, []uint64{10000, 1000, 1001, 1002}, []float64{0.2, 0.5, 1}),
		),
		makeSummaryMetric("rpc_duration_seconds", startTs1,
			makeSummaryPoint(ts1, 900, 8000, map[float64]float64{1: 31, 5: 35, 50: 47, 90: 70, 99: 76}, kv{"foo", "bar"}),
			makeSummaryPoint(ts1, 50, 100, nil, kv{"foo", "no_quantile"}),
		),
		/*
			makeGaugeMetric("up", startTs1, makeDoublePoint(ts1, 1.0)),
			makeGaugeMetric("scrape_duration_seconds", startTs1, makeDoublePoint(ts1, 0.001662471)),
			makeGaugeMetric("scrape_samples_scraped", startTs1, makeDoublePoint(ts1, 18.0)),
			makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs1, makeDoublePoint(ts1, 18.0)),
			makeGaugeMetric("scrape_series_added", startTs1, makeDoublePoint(ts1, 18.0)),
		*/
	)
	doCompare(t, "scrape1", want1, got1)

	// Verify the 2nd data.
	got2 := metricsL[1]
	startTs2, ts2 := extractFirstTimestamp(got2)
	want2 := makeMetrics(
		td,
		makeGaugeMetric("go_threads", startTs2, makeDoublePoint(ts2, 16.0)),
		makeCumulativeDistMetric("http_request_duration_seconds", startTs2,
			makeDistPoint(ts2, 14003, 50100, []uint64{11000, 1000, 1001, 1002}, []float64{0.2, 0.5, 1}),
		),
		makeSummaryMetric("rpc_duration_seconds", startTs2,
			makeSummaryPoint(ts2, 950, 8100, map[float64]float64{1: 32, 5: 35, 50: 47, 90: 70, 99: 77}, kv{"foo", "bar"}),
			makeSummaryPoint(ts1, 55, 101, nil, kv{"foo", "no_quantile"}),
		),
		makeGaugeMetric("up", startTs1, makeDoublePoint(ts1, 1.0)),
		makeGaugeMetric("scrape_duration_seconds", startTs1, makeDoublePoint(ts1, 0.001662471)),
		makeGaugeMetric("scrape_samples_scraped", startTs1, makeDoublePoint(ts1, 18.0)),
		makeGaugeMetric("scrape_samples_post_metric_relabeling", startTs1, makeDoublePoint(ts1, 18.0)),
		makeGaugeMetric("scrape_series_added", startTs1, makeDoublePoint(ts1, 18.0)),
	)
	doCompare(t, "scrape2", want2, got2)
}

// TestEndToEnd  end to end test executor
func TestEndToEnd(t *testing.T) {
	// 1. setup input data
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: target1Page1},
				{code: 500, data: ""},
				{code: 200, data: target1Page2},
			},
			validateFunc: verifyTarget1,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: target2Page1},
				{code: 200, data: target2Page2},
				{code: 500, data: ""},
				{code: 200, data: target2Page3},
				{code: 200, data: target2Page4},
				{code: 500, data: ""},
				{code: 200, data: target2Page5},
			},
			validateFunc: verifyTarget2,
		},
		{
			name: "target3",
			pages: []mockPrometheusResponse{
				{code: 200, data: target3Page1},
				{code: 200, data: target3Page2},
			},
			validateFunc: verifyTarget3,
		},
	}

	testEndToEnd(t, targets, false)
}

var startTimeMetricPage = `
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
# HELP process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE process_start_time_seconds gauge
process_start_time_seconds 400.8
`

var startTimeMetricPageStartTimestamp = &timestamppb.Timestamp{Seconds: 400, Nanos: 800000000}

// 6 metrics + 5 internal metrics
const numStartTimeMetricPageTimeseries = 11

const timestampZero pdata.Timestamp = 0

func verifyStartTimeMetricPage(t *testing.T, _ *testData, metricsL []pdata.ResourceMetrics) {
	numTimeseries := 0
	for _, rms := range metricsL {
		_, ts, dataType := extractFirstTimestampDataType(rms)
		want := pdata.NewTimestampFromTime(time.Unix(400, 800000000))
		switch dataType {
		case pdata.MetricDataTypeGauge:
			want = timestampZero

		case pdata.MetricDataTypeHistogram:
			histogram := rms.InstrumentationLibraryMetrics().At(0).Metrics().At(0).Histogram()
			if histogram.AggregationTemporality() == pdata.AggregationTemporalityDelta {
				want = timestampZero
			}
		}
		assert.Equal(t, want, ts, "Timestamp mismatch")
		numTimeseries++
	}

	assert.Equal(t, numStartTimeMetricPageTimeseries, numTimeseries)
}

// TestStartTimeMetric validates that timeseries have start time set to 'process_start_time_seconds'
func TestStartTimeMetric(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: startTimeMetricPage},
			},
			validateFunc: verifyStartTimeMetricPage,
		},
	}
	testEndToEnd(t, targets, true)
}

func testEndToEnd(t *testing.T, targets []*testData, useStartTimeMetric bool) {
	// 1. setup mock server
	mp, cfg, err := setupMockPrometheus(targets...)
	require.Nilf(t, err, "Failed to create Promtheus config: %v", err)
	defer mp.Close()

	cms := new(consumertest.MetricsSink)
	rcvr := newPrometheusReceiver(logger, &Config{
		ReceiverSettings:   config.NewReceiverSettings(config.NewID(typeStr)),
		PrometheusConfig:   cfg,
		UseStartTimeMetric: useStartTimeMetric}, cms)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()), "Failed to invoke Start: %v", err)
	t.Cleanup(func() {
		// verify state after shutdown is called
		assert.Lenf(t, flattenTargets(rcvr.scrapeManager.TargetsAll()), len(targets), "expected %v targets to be running", len(targets))
		require.NoError(t, rcvr.Shutdown(context.Background()))
		assert.Len(t, flattenTargets(rcvr.scrapeManager.TargetsAll()), 0, "expected scrape manager to have no targets")
	})

	// wait for all provided data to be scraped
	mp.wg.Wait()

	// split and store results by target name
	results := metricsGroupedByServiceName(t, cms.AllMetrics())

	lres, lep := len(results), len(mp.endpoints)
	assert.Equalf(t, lep, lres, "want %d targets, but got %v\n", lep, lres)

	// Skipping the validate loop below, because it falsely assumed that
	// staleness markers would not be returned, yet the tests are a bit rigid.
	if false {
		t.Log(`Skipping the "up" metric checks as they seem to be spuriously failing after staleness marker insertions`)
		return
	}

	// loop to validate outputs for each targets
	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			mds := getValidScrapes(t, results[target.name])
			target.validateFunc(t, target, mds)
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

var startTimeMetricRegexPage = `
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
# HELP example_process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE example_process_start_time_seconds gauge
example_process_start_time_seconds 400.8
`

// TestStartTimeMetricRegex validates that timeseries have start time regex set to 'process_start_time_seconds'
func TestStartTimeMetricRegex(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: startTimeMetricRegexPage},
			},
			validateFunc: verifyStartTimeMetricPage,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: startTimeMetricPage},
			},
			validateFunc: verifyStartTimeMetricPage,
		},
	}
	// Splitting out targets, because the prior tests were oblivious
	// about staleness metrics being emitted, and hence when trying
	// to compare values across 2 different scrapes emits staleness
	// markers whose NaN values are unaccounted for.
	// TODO: Perhaps refactor these tests.
	for _, target := range targets {
		testEndToEndRegex(t, []*testData{target}, true, "^(.+_)*process_start_time_seconds$")
	}
}

func metricsGroupedByServiceName(t *testing.T, metricsL []pdata.Metrics) map[string][]pdata.ResourceMetrics {
	// split and store results by target name
	splits := make(map[string][]pdata.ResourceMetrics)
	for _, metrics := range metricsL {
		rms := metrics.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			rmi := rms.At(i)
			serviceNameAttr, ok := rmi.Resource().Attributes().Get("service.name")
			assert.True(t, ok, `expected "service.name" as a known attribute`)
			serviceName := serviceNameAttr.StringVal()
			splits[serviceName] = append(splits[serviceName], rmi)
		}
	}
	return splits
}

func testEndToEndRegex(t *testing.T, targets []*testData, useStartTimeMetric bool, startTimeMetricRegex string) {
	// 1. setup mock server
	mp, cfg, err := setupMockPrometheus(targets...)
	require.Nilf(t, err, "Failed to create Promtheus config: %v", err)
	defer mp.Close()

	cms := new(consumertest.MetricsSink)
	rcvr := newPrometheusReceiver(logger, &Config{
		ReceiverSettings:     config.NewReceiverSettings(config.NewID(typeStr)),
		PrometheusConfig:     cfg,
		UseStartTimeMetric:   useStartTimeMetric,
		StartTimeMetricRegex: startTimeMetricRegex}, cms)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()), "Failed to invoke Start: %v", err)
	t.Cleanup(func() { require.NoError(t, rcvr.Shutdown(context.Background())) })

	// wait for all provided data to be scraped
	mp.wg.Wait()
	results := metricsGroupedByServiceName(t, cms.AllMetrics())

	lres, lep := len(results), len(mp.endpoints)
	assert.Equalf(t, lep, lres, "want %d targets, but got %v\n", lep, lres)

	// loop to validate outputs for each targets
	for _, target := range targets {
		target.validateFunc(t, target, results[target.name])
	}
}
