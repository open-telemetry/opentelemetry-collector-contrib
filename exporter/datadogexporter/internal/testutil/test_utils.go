// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package testutil contains the test util functions
package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/DataDog/sketches-go/ddsketch"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/protobuf/proto"
)

var (
	testAttributes = map[string]string{"datadog.host.name": "custom-hostname"}
	// TestMetrics metrics for tests.
	TestMetrics = newMetricsWithAttributeMap(testAttributes)
	// TestTraces traces for tests.
	TestTraces = newTracesWithAttributeMap(testAttributes)
)

type DatadogServer struct {
	*httptest.Server
	MetadataChan chan []byte
}

/* #nosec G101 -- This is a false positive, these are API endpoints rather than credentials */
const (
	ValidateAPIKeyEndpoint = "/api/v1/validate" // nolint G101
	MetricV1Endpoint       = "/api/v1/series"
	MetricV2Endpoint       = "/api/v2/series"
	SketchesMetricEndpoint = "/api/beta/sketches"
	MetadataEndpoint       = "/intake"
	TraceEndpoint          = "/api/v0.2/traces"
	APMStatsEndpoint       = "/api/v0.2/stats"
)

// DatadogServerMock mocks a Datadog backend server
func DatadogServerMock(overwriteHandlerFuncs ...OverwriteHandleFunc) *DatadogServer {
	metadataChan := make(chan []byte)
	mux := http.NewServeMux()

	handlers := map[string]http.HandlerFunc{
		ValidateAPIKeyEndpoint: validateAPIKeyEndpoint,
		MetricV1Endpoint:       metricsEndpoint,
		MetricV2Endpoint:       metricsV2Endpoint,
		MetadataEndpoint:       newMetadataEndpoint(metadataChan),
		"/":                    func(w http.ResponseWriter, r *http.Request) {},
	}
	for _, f := range overwriteHandlerFuncs {
		p, hf := f()
		handlers[p] = hf
	}
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}

	srv := httptest.NewServer(mux)

	return &DatadogServer{
		srv,
		metadataChan,
	}
}

// OverwriteHandleFuncs allows to overwrite the default handler functions
type OverwriteHandleFunc func() (string, http.HandlerFunc)

// HTTPRequestRecorder records a HTTP request.
type HTTPRequestRecorder struct {
	Pattern  string
	Header   http.Header
	ByteBody []byte
}

func (rec *HTTPRequestRecorder) HandlerFunc() (string, http.HandlerFunc) {
	return rec.Pattern, func(w http.ResponseWriter, r *http.Request) {
		rec.Header = r.Header
		rec.ByteBody, _ = io.ReadAll(r.Body)
	}
}

// HTTPRequestRecorderWithChan puts all incoming HTTP request bytes to the given channel.
type HTTPRequestRecorderWithChan struct {
	Pattern string
	ReqChan chan []byte
}

func (rec *HTTPRequestRecorderWithChan) HandlerFunc() (string, http.HandlerFunc) {
	return rec.Pattern, func(w http.ResponseWriter, r *http.Request) {
		bytesBody, _ := io.ReadAll(r.Body)
		rec.ReqChan <- bytesBody
	}
}

// ValidateAPIKeyEndpointInvalid returns a handler function that returns an invalid API key response
func ValidateAPIKeyEndpointInvalid() (string, http.HandlerFunc) {
	return "/api/v1/validate", validateAPIKeyEndpointInvalid
}

type validateAPIKeyResponse struct {
	Valid bool `json:"valid"`
}

func validateAPIKeyEndpoint(w http.ResponseWriter, _ *http.Request) {
	res := validateAPIKeyResponse{Valid: true}
	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(resJSON)
	if err != nil {
		log.Fatalln(err)
	}
}

func validateAPIKeyEndpointInvalid(w http.ResponseWriter, _ *http.Request) {
	res := validateAPIKeyResponse{Valid: false}
	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(resJSON)
	if err != nil {
		log.Fatalln(err)
	}
}

type metricsResponse struct {
	Status string `json:"status"`
}

func metricsEndpoint(w http.ResponseWriter, _ *http.Request) {
	res := metricsResponse{Status: "ok"}
	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, err := w.Write(resJSON)
	if err != nil {
		log.Fatalln(err)
	}
}

func metricsV2Endpoint(w http.ResponseWriter, _ *http.Request) {
	res := metricsResponse{Status: "ok"}
	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, err := w.Write(resJSON)
	if err != nil {
		log.Fatalln(err)
	}
}

func newMetadataEndpoint(c chan []byte) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c <- body
	}
}

func fillAttributeMap(attrs pcommon.Map, mp map[string]string) {
	attrs.EnsureCapacity(len(mp))
	for k, v := range mp {
		attrs.PutStr(k, v)
	}
}

// NewAttributeMap creates a new attribute map (string only)
// from a Go map
func NewAttributeMap(mp map[string]string) pcommon.Map {
	attrs := pcommon.NewMap()
	fillAttributeMap(attrs, mp)
	return attrs
}

// TestGauge holds the definition of a basic gauge.
type TestGauge struct {
	Name       string
	DataPoints []DataPoint
}

// DataPoint specifies a DoubleVal data point and its attributes.
type DataPoint struct {
	Value      float64
	Attributes map[string]string
}

// NewGaugeMetrics creates a set of pmetric.Metrics containing all the specified
// test gauges.
func NewGaugeMetrics(tgs []TestGauge) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	all := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	for _, tg := range tgs {
		m := all.AppendEmpty()
		m.SetName(tg.Name)
		g := m.SetEmptyGauge()
		for _, dp := range tg.DataPoints {
			d := g.DataPoints().AppendEmpty()
			d.SetDoubleValue(dp.Value)
			fillAttributeMap(d.Attributes(), dp.Attributes)
		}
	}
	return metrics
}

func newMetricsWithAttributeMap(mp map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	fillAttributeMap(md.ResourceMetrics().AppendEmpty().Resource().Attributes(), mp)
	return md
}

func newTracesWithAttributeMap(mp map[string]string) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()
	rs := resourceSpans.AppendEmpty()
	fillAttributeMap(rs.Resource().Attributes(), mp)
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	return traces
}

type MockSourceProvider struct {
	Src source.Source
}

func (s *MockSourceProvider) Source(_ context.Context) (source.Source, error) {
	return s.Src, nil
}

type MockStatsProcessor struct {
	In []*pb.ClientStatsPayload
}

func (s *MockStatsProcessor) ProcessStats(in *pb.ClientStatsPayload, _, _ string) {
	s.In = append(s.In, in)
}

// StatsPayloads contains a couple of *pb.ClientStatsPayloads used for testing.
var StatsPayloads = []*pb.ClientStatsPayload{
	{
		Hostname:         "host",
		Env:              "prod",
		Version:          "v1.2",
		Lang:             "go",
		TracerVersion:    "v44",
		RuntimeID:        "123jkl",
		Sequence:         2,
		AgentAggregation: "blah",
		Service:          "mysql",
		ContainerID:      "abcdef123456",
		Tags:             []string{"a:b", "c:d"},
		Stats: []*pb.ClientStatsBucket{
			{
				Start:    10,
				Duration: 1,
				Stats: []*pb.ClientGroupedStats{
					{
						Service:        "kafka",
						Name:           "queue.add",
						Resource:       "append",
						HTTPStatusCode: 220,
						Type:           "queue",
						Hits:           15,
						Errors:         3,
						Duration:       143,
						OkSummary:      testSketchBytes(1, 2, 3),
						ErrorSummary:   testSketchBytes(4, 5, 6),
						TopLevelHits:   5,
					},
				},
			},
		},
	},
	{
		Hostname:         "host2",
		Env:              "prod2",
		Version:          "v1.22",
		Lang:             "go2",
		TracerVersion:    "v442",
		RuntimeID:        "123jkl2",
		Sequence:         22,
		AgentAggregation: "blah2",
		Service:          "mysql2",
		ContainerID:      "abcdef1234562",
		Tags:             []string{"a:b2", "c:d2"},
		Stats: []*pb.ClientStatsBucket{
			{
				Start:    102,
				Duration: 12,
				Stats: []*pb.ClientGroupedStats{
					{
						Service:        "kafka2",
						Name:           "queue.add2",
						Resource:       "append2",
						HTTPStatusCode: 2202,
						Type:           "queue2",
						Hits:           152,
						Errors:         32,
						Duration:       1432,
						OkSummary:      testSketchBytes(7, 8),
						ErrorSummary:   testSketchBytes(9, 10, 11),
						TopLevelHits:   52,
					},
				},
			},
		},
	},
}

// The sketch's relative accuracy and maximum number of bins is identical
// to the one used in the trace-agent for consistency:
// https://github.com/DataDog/datadog-agent/blob/cbac965/pkg/trace/stats/statsraw.go#L18-L26
const (
	sketchRelativeAccuracy = 0.01
	sketchMaxBins          = 2048
)

// testSketchBytes returns the proto-encoded version of a DDSketch containing the
// points in nums.
func testSketchBytes(nums ...float64) []byte {
	sketch, err := ddsketch.LogCollapsingLowestDenseDDSketch(sketchRelativeAccuracy, sketchMaxBins)
	if err != nil {
		// the only possible error is if the relative accuracy is < 0 or > 1;
		// we know that's not the case because it's a constant defined as 0.01
		panic(err)
	}
	for _, num := range nums {
		if err2 := sketch.Add(num); err2 != nil {
			panic(err2)
		}
	}
	buf, err := proto.Marshal(sketch.ToProto())
	if err != nil {
		// there should be no error under any circumstances here
		panic(err)
	}
	return buf
}
