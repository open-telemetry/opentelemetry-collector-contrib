// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package newrelicexporter

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	testCollectorName    = "TestCollector"
	testCollectorVersion = "v1.2.3"
)

type mockConfig struct {
	useAPIKeyHeader bool
	serverURL       string
	statusCode      int
}

func runTraceMock(initialContext context.Context, ptrace pdata.Traces, cfg mockConfig) (*Mock, error) {
	ctx, cancel := context.WithCancel(initialContext)
	defer cancel()

	m := &Mock{
		Batches:    make([]Batch, 0, 1),
		StatusCode: 202,
	}

	if cfg.statusCode > 0 {
		m.StatusCode = cfg.statusCode
	}

	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	urlString := srv.URL
	if cfg.serverURL != "" {
		urlString = cfg.serverURL
	}
	u, _ := url.Parse(urlString)

	if cfg.useAPIKeyHeader {
		c.CommonConfig.APIKeyHeader = "api-key"
	} else {
		c.CommonConfig.APIKey = "NRII-1"
	}
	c.TracesConfig.insecure, c.TracesConfig.HostOverride = true, u.Host
	params := component.ExporterCreateSettings{Logger: zap.NewNop(), BuildInfo: component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}}
	exp, err := f.CreateTracesExporter(context.Background(), params, c)
	if err != nil {
		return m, err
	}
	if err := exp.ConsumeTraces(ctx, ptrace); err != nil {
		return m, err
	}
	if err := exp.Shutdown(ctx); err != nil {
		return m, err
	}
	return m, nil
}

func runMetricMock(initialContext context.Context, pmetrics pdata.Metrics, cfg mockConfig) (*Mock, error) {
	ctx, cancel := context.WithCancel(initialContext)
	defer cancel()

	m := &Mock{
		Batches:    make([]Batch, 0, 1),
		StatusCode: 202,
	}

	if cfg.statusCode > 0 {
		m.StatusCode = cfg.statusCode
	}

	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	urlString := srv.URL
	if cfg.serverURL != "" {
		urlString = cfg.serverURL
	}
	u, _ := url.Parse(urlString)

	if cfg.useAPIKeyHeader {
		c.CommonConfig.APIKeyHeader = "api-key"
	} else {
		c.CommonConfig.APIKey = "NRII-1"
	}
	c.MetricsConfig.insecure, c.MetricsConfig.HostOverride = true, u.Host
	params := component.ExporterCreateSettings{Logger: zap.NewNop(), BuildInfo: component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}}
	exp, err := f.CreateMetricsExporter(context.Background(), params, c)
	if err != nil {
		return m, err
	}
	if err := exp.ConsumeMetrics(ctx, pmetrics); err != nil {
		return m, err
	}
	if err := exp.Shutdown(ctx); err != nil {
		return m, err
	}
	return m, nil
}

func runLogMock(initialContext context.Context, plogs pdata.Logs, cfg mockConfig) (*Mock, error) {
	ctx, cancel := context.WithCancel(initialContext)
	defer cancel()

	m := &Mock{
		Batches:    make([]Batch, 0, 1),
		StatusCode: 202,
	}

	if cfg.statusCode > 0 {
		m.StatusCode = cfg.statusCode
	}

	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	urlString := srv.URL
	if cfg.serverURL != "" {
		urlString = cfg.serverURL
	}
	u, _ := url.Parse(urlString)

	if cfg.useAPIKeyHeader {
		c.CommonConfig.APIKeyHeader = "api-key"
	} else {
		c.CommonConfig.APIKey = "NRII-1"
	}
	c.LogsConfig.insecure, c.LogsConfig.HostOverride = true, u.Host
	params := component.ExporterCreateSettings{Logger: zap.NewNop(), BuildInfo: component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}}
	exp, err := f.CreateLogsExporter(context.Background(), params, c)
	if err != nil {
		return m, err
	}
	if err := exp.ConsumeLogs(ctx, plogs); err != nil {
		return m, err
	}
	if err := exp.Shutdown(ctx); err != nil {
		return m, err
	}
	return m, nil
}

func testTraceData(t *testing.T, expected []Batch, resource *resourcepb.Resource, spans []*tracepb.Span, apiKey string) {
	ctx := context.Background()
	useAPIKeyHeader := apiKey != ""
	if useAPIKeyHeader {
		ctx = metadata.NewIncomingContext(ctx, metadata.MD{"api-key": []string{apiKey}})
	}

	m, err := runTraceMock(ctx, internaldata.OCToTraces(nil, resource, spans), mockConfig{useAPIKeyHeader: useAPIKeyHeader})
	require.NoError(t, err)
	assert.Equal(t, expected, m.Batches)
	if !useAPIKeyHeader {
		assert.Equal(t, []string{"NRII-1"}, m.Header[http.CanonicalHeaderKey("api-key")])
	} else if len(apiKey) < 40 {
		assert.Equal(t, []string{apiKey}, m.Header[http.CanonicalHeaderKey("api-key")])
	} else {
		assert.Equal(t, []string{apiKey}, m.Header[http.CanonicalHeaderKey("x-license-key")])
	}
}

func testMetricData(t *testing.T, expected []Batch, md *agentmetricspb.ExportMetricsServiceRequest, apiKey string) {
	ctx := context.Background()
	useAPIKeyHeader := apiKey != ""
	if useAPIKeyHeader {
		ctx = metadata.NewIncomingContext(ctx, metadata.MD{"api-key": []string{apiKey}})
	}

	m, err := runMetricMock(ctx, internaldata.OCToMetrics(md.Node, md.Resource, md.Metrics), mockConfig{useAPIKeyHeader: useAPIKeyHeader})
	require.NoError(t, err)
	assert.Equal(t, expected, m.Batches)
}

func testLogData(t *testing.T, expected []Batch, logs pdata.Logs, apiKey string) {
	ctx := context.Background()
	useAPIKeyHeader := apiKey != ""
	if useAPIKeyHeader {
		ctx = metadata.NewIncomingContext(ctx, metadata.MD{"api-key": []string{apiKey}})
	}

	l, err := runLogMock(ctx, logs, mockConfig{useAPIKeyHeader: useAPIKeyHeader})
	require.NoError(t, err)
	assert.Equal(t, expected, l.Batches)
}

func TestExportTraceWithBadURL(t *testing.T) {
	ptrace := internaldata.OCToTraces(nil, nil,
		[]*tracepb.Span{
			{
				SpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:   &tracepb.TruncatableString{Value: "a"},
			},
		})

	_, err := runTraceMock(context.Background(), ptrace, mockConfig{serverURL: "http://badurl"})
	require.Error(t, err)
}

func TestExportTraceWithErrorStatusCode(t *testing.T) {
	ptrace := internaldata.OCToTraces(nil, nil,
		[]*tracepb.Span{
			{
				SpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:   &tracepb.TruncatableString{Value: "a"},
			},
		})

	_, err := runTraceMock(context.Background(), ptrace, mockConfig{statusCode: 500})
	require.Error(t, err)
}

func TestExportTraceWithNot202StatusCode(t *testing.T) {
	ptrace := internaldata.OCToTraces(nil, nil,
		[]*tracepb.Span{
			{
				SpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:   &tracepb.TruncatableString{Value: "a"},
			},
		})

	{
		_, err := runTraceMock(context.Background(), ptrace, mockConfig{statusCode: 403})
		require.Error(t, err)
	}
	{
		_, err := runTraceMock(context.Background(), ptrace, mockConfig{statusCode: 429})
		require.Error(t, err)
	}
}

func TestExportTraceWithBadPayload(t *testing.T) {
	ptrace := internaldata.OCToTraces(nil, nil,
		[]*tracepb.Span{
			{
				SpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:   &tracepb.TruncatableString{Value: "a"},
			},
		})

	_, err := runTraceMock(context.Background(), ptrace, mockConfig{statusCode: 400})
	require.Error(t, err)
}

func TestExportTraceWithInvalidMetadata(t *testing.T) {
	ptrace := internaldata.OCToTraces(nil, nil,
		[]*tracepb.Span{
			{
				SpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:   &tracepb.TruncatableString{Value: "a"},
			},
		})

	_, err := runTraceMock(context.Background(), ptrace, mockConfig{useAPIKeyHeader: true})
	require.Error(t, err)
}

func TestExportTraceWithNoAPIKeyInMetadata(t *testing.T) {
	ptrace := internaldata.OCToTraces(nil, nil,
		[]*tracepb.Span{
			{
				SpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:   &tracepb.TruncatableString{Value: "a"},
			},
		})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
	_, err := runTraceMock(ctx, ptrace, mockConfig{useAPIKeyHeader: true})
	require.Error(t, err)
}

func TestExportTracePartialData(t *testing.T) {
	ptrace := internaldata.OCToTraces(nil, nil,
		[]*tracepb.Span{
			{
				SpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
				Name:   &tracepb.TruncatableString{Value: "no trace id"},
			},
			{
				TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				Name:    &tracepb.TruncatableString{Value: "no span id"},
			},
		})

	_, err := runTraceMock(context.Background(), ptrace, mockConfig{useAPIKeyHeader: false})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), errInvalidSpanID.Error()))
	assert.True(t, strings.Contains(err.Error(), errInvalidTraceID.Error()))
}

func TestExportTraceDataMinimum(t *testing.T) {
	spans := []*tracepb.Span{
		{
			TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
			Name:    &tracepb.TruncatableString{Value: "root"},
		},
	}

	expected := []Batch{
		{
			Common: Common{
				Attributes: map[string]string{
					"collector.name":    testCollectorName,
					"collector.version": testCollectorVersion,
				},
			},
			Spans: []Span{
				{
					ID:      "0000000000000001",
					TraceID: "01010101010101010101010101010101",
					Attributes: map[string]interface{}{
						"name": "root",
					},
				},
			},
		},
	}

	testTraceData(t, expected, nil, spans, "")
	testTraceData(t, expected, nil, spans, "0000000000000000000000000000000000000000")
	testTraceData(t, expected, nil, spans, "NRII-api-key")
}

func TestExportTraceDataFullTrace(t *testing.T) {
	resource := &resourcepb.Resource{
		Labels: map[string]string{
			"service.name": "test-service",
			"resource":     "R1",
		},
	}

	spans := []*tracepb.Span{
		{
			TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 1},
			Name:    &tracepb.TruncatableString{Value: "root"},
			Status:  &tracepb.Status{},
		},
		{
			TraceId:      []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			SpanId:       []byte{0, 0, 0, 0, 0, 0, 0, 2},
			ParentSpanId: []byte{0, 0, 0, 0, 0, 0, 0, 1},
			Name:         &tracepb.TruncatableString{Value: "client"},
			Status:       &tracepb.Status{},
		},
		{
			TraceId:      []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			SpanId:       []byte{0, 0, 0, 0, 0, 0, 0, 3},
			ParentSpanId: []byte{0, 0, 0, 0, 0, 0, 0, 2},
			Name:         &tracepb.TruncatableString{Value: "server"},
			Status:       &tracepb.Status{},
		},
	}

	expected := []Batch{
		{
			Common: Common{
				Attributes: map[string]string{
					"collector.name":    testCollectorName,
					"collector.version": testCollectorVersion,
					"resource":          "R1",
					"service.name":      "test-service",
				},
			},
			Spans: []Span{
				{
					ID:      "0000000000000001",
					TraceID: "01010101010101010101010101010101",
					Attributes: map[string]interface{}{
						"name": "root",
					},
				},
				{
					ID:      "0000000000000002",
					TraceID: "01010101010101010101010101010101",
					Attributes: map[string]interface{}{
						"name":      "client",
						"parent.id": "0000000000000001",
					},
				},
				{
					ID:      "0000000000000003",
					TraceID: "01010101010101010101010101010101",
					Attributes: map[string]interface{}{
						"name":      "server",
						"parent.id": "0000000000000002",
					},
				},
			},
		},
	}

	testTraceData(t, expected, resource, spans, "")
	testTraceData(t, expected, resource, spans, "0000000000000000000000000000000000000000")
	testTraceData(t, expected, resource, spans, "NRII-api-key")
}

func TestExportMetricUnsupported(t *testing.T) {
	ms := pdata.NewMetrics()
	m := ms.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetDataType(pdata.MetricDataTypeHistogram)
	dp := m.Histogram().DataPoints().AppendEmpty()
	dp.SetCount(1)
	dp.SetSum(1)
	dp.SetTimestamp(pdata.TimestampFromTime(time.Now()))

	_, err := runMetricMock(context.Background(), ms, mockConfig{useAPIKeyHeader: false})
	var unsupportedErr *errUnsupportedMetricType
	assert.ErrorAs(t, err, &unsupportedErr, "error was not the expected unsupported metric type error")
}

func TestExportMetricDataMinimal(t *testing.T) {
	desc := "physical property of matter that quantitatively expresses hot and cold"
	unit := "K"
	md := &agentmetricspb.ExportMetricsServiceRequest{
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "temperature",
					Description: desc,
					Unit:        unit,
					Type:        metricspb.MetricDescriptor_GAUGE_DOUBLE,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "location"},
						{Key: "elevation"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "Portland", HasValue: true},
							{Value: "0", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamppb.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 293.15,
								},
							},
						},
					},
				},
			},
		},
	}

	expected := []Batch{
		{
			Common: Common{
				Attributes: map[string]string{
					"collector.name":    testCollectorName,
					"collector.version": testCollectorVersion,
				},
			},
			Metrics: []Metric{
				{
					Name:      "temperature",
					Type:      "gauge",
					Value:     293.15,
					Timestamp: int64(100 * time.Microsecond),
					Attributes: map[string]interface{}{
						"description": desc,
						"unit":        unit,
						"location":    "Portland",
						"elevation":   "0",
					},
				},
			},
		},
	}

	testMetricData(t, expected, md, "NRII-api-key")
	testMetricData(t, expected, md, "0000000000000000000000000000000000000000")
	testMetricData(t, expected, md, "")
}

func TestExportMetricDataFull(t *testing.T) {
	desc := "physical property of matter that quantitatively expresses hot and cold"
	unit := "K"
	md := &agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-service"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "temperature",
					Description: desc,
					Unit:        unit,
					Type:        metricspb.MetricDescriptor_GAUGE_DOUBLE,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "location"},
						{Key: "elevation"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "Portland", HasValue: true},
							{Value: "0", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamppb.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 293.15,
								},
							},
							{
								Timestamp: &timestamppb.Timestamp{
									Seconds: 101,
								},
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 293.15,
								},
							},
							{
								Timestamp: &timestamppb.Timestamp{
									Seconds: 102,
								},
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 293.45,
								},
							},
						},
					},
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "Denver", HasValue: true},
							{Value: "5280", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamppb.Timestamp{
									Seconds: 99,
								},
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 290.05,
								},
							},
							{
								Timestamp: &timestamppb.Timestamp{
									Seconds: 106,
								},
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 293.15,
								},
							},
						},
					},
				},
			},
		},
	}

	expected := []Batch{
		{
			Common: Common{
				Attributes: map[string]string{
					"collector.name":    testCollectorName,
					"collector.version": testCollectorVersion,
					"resource":          "R1",
					"service.name":      "test-service",
				},
			},
			Metrics: []Metric{
				{
					Name:      "temperature",
					Type:      "gauge",
					Value:     293.15,
					Timestamp: int64(100 * time.Microsecond),
					Attributes: map[string]interface{}{
						"description": desc,
						"unit":        unit,
						"location":    "Portland",
						"elevation":   "0",
					},
				},
				{
					Name:      "temperature",
					Type:      "gauge",
					Value:     293.15,
					Timestamp: int64(101 * time.Microsecond),
					Attributes: map[string]interface{}{
						"description": desc,
						"unit":        unit,
						"location":    "Portland",
						"elevation":   "0",
					},
				},
				{
					Name:      "temperature",
					Type:      "gauge",
					Value:     293.45,
					Timestamp: int64(102 * time.Microsecond),
					Attributes: map[string]interface{}{
						"description": desc,
						"unit":        unit,
						"location":    "Portland",
						"elevation":   "0",
					},
				},
				{
					Name:      "temperature",
					Type:      "gauge",
					Value:     290.05,
					Timestamp: int64(99 * time.Microsecond),
					Attributes: map[string]interface{}{
						"description": desc,
						"unit":        unit,
						"location":    "Denver",
						"elevation":   "5280",
					},
				},
				{
					Name:      "temperature",
					Type:      "gauge",
					Value:     293.15,
					Timestamp: int64(106 * time.Microsecond),
					Attributes: map[string]interface{}{
						"description": desc,
						"unit":        unit,
						"location":    "Denver",
						"elevation":   "5280",
					},
				},
			},
		},
	}

	testMetricData(t, expected, md, "")
	testMetricData(t, expected, md, "0000000000000000000000000000000000000000")
	testMetricData(t, expected, md, "NRII-api-key")
}

func TestExportLogs(t *testing.T) {
	timestamp := time.Now()
	logs := pdata.NewLogs()
	rlog := logs.ResourceLogs().AppendEmpty()
	rlog.Resource().Attributes().InsertString("resource", "R1")
	rlog.Resource().Attributes().InsertString("service.name", "test-service")
	l := rlog.InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()
	l.SetName("logname")
	l.SetTimestamp(pdata.TimestampFromTime(timestamp))
	l.Body().SetStringVal("log body")
	l.Attributes().InsertString("foo", "bar")

	expected := []Batch{
		{
			Common: Common{
				Attributes: map[string]string{
					"collector.name":    testCollectorName,
					"collector.version": testCollectorVersion,
					"resource":          "R1",
					"service.name":      "test-service",
				},
			},
			Logs: []Log{
				{
					Message:   "log body",
					Timestamp: timestamp.UnixNano() / (1000 * 1000),
					Attributes: map[string]interface{}{
						"foo":  "bar",
						"name": "logname",
					},
				},
			},
		},
	}

	testLogData(t, expected, logs, "")
	testLogData(t, expected, logs, "0000000000000000000000000000000000000000")
	testLogData(t, expected, logs, "NRII-api-key")
}

func TestCreatesClientOptionWithVersionInUserAgent(t *testing.T) {
	testUserAgentContainsCollectorInfo(t, testCollectorVersion, testCollectorName, "NewRelic-OpenTelemetry-Collector/v1.2.3 TestCollector")
}

func testUserAgentContainsCollectorInfo(t *testing.T, version string, exeName string, expectedUserAgentSubstring string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &Mock{
		Batches:    make([]Batch, 0, 1),
		StatusCode: 202,
	}

	cfg := mockConfig{useAPIKeyHeader: false}

	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	urlString := srv.URL
	if cfg.serverURL != "" {
		urlString = cfg.serverURL
	}
	u, _ := url.Parse(urlString)

	if cfg.useAPIKeyHeader {
		c.CommonConfig.APIKeyHeader = "api-key"
	} else {
		c.CommonConfig.APIKey = "NRII-1"
	}
	c.TracesConfig.insecure, c.TracesConfig.HostOverride = true, u.Host
	params := component.ExporterCreateSettings{Logger: zap.NewNop(), BuildInfo: component.BuildInfo{
		Command: exeName,
		Version: version,
	}}
	exp, err := f.CreateTracesExporter(context.Background(), params, c)
	require.NoError(t, err)

	ptrace := pdata.NewTraces()
	s := ptrace.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	s.SetName("root")
	s.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	s.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))

	err = exp.ConsumeTraces(ctx, ptrace)
	require.NoError(t, err)
	err = exp.Shutdown(ctx)
	require.NoError(t, err)

	assert.Contains(t, m.Header[http.CanonicalHeaderKey("user-agent")][0], expectedUserAgentSubstring)
}

func TestBadSpanResourceGeneratesError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &Mock{
		Batches:    make([]Batch, 0, 1),
		StatusCode: 202,
	}

	cfg := mockConfig{useAPIKeyHeader: false}

	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	urlString := srv.URL
	if cfg.serverURL != "" {
		urlString = cfg.serverURL
	}
	u, _ := url.Parse(urlString)

	if cfg.useAPIKeyHeader {
		c.CommonConfig.APIKeyHeader = "api-key"
	} else {
		c.CommonConfig.APIKey = "NRII-1"
	}
	c.TracesConfig.insecure, c.TracesConfig.HostOverride = true, u.Host
	params := component.ExporterCreateSettings{Logger: zap.NewNop(), BuildInfo: component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}}
	exp, err := f.CreateTracesExporter(context.Background(), params, c)
	require.NoError(t, err)

	ptrace := pdata.NewTraces()
	rs := ptrace.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().InsertDouble("badattribute", math.Inf(1))
	s := rs.InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	s.SetName("root")
	s.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	s.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))

	errorFromConsumeTraces := exp.ConsumeTraces(ctx, ptrace)

	err = exp.Shutdown(ctx)
	require.NoError(t, err)

	require.Error(t, errorFromConsumeTraces)
}

func TestBadMetricResourceGeneratesError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &Mock{
		Batches:    make([]Batch, 0, 1),
		StatusCode: 202,
	}

	cfg := mockConfig{useAPIKeyHeader: false}

	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	urlString := srv.URL
	if cfg.serverURL != "" {
		urlString = cfg.serverURL
	}
	u, _ := url.Parse(urlString)

	if cfg.useAPIKeyHeader {
		c.CommonConfig.APIKeyHeader = "api-key"
	} else {
		c.CommonConfig.APIKey = "NRII-1"
	}
	c.TracesConfig.insecure, c.TracesConfig.HostOverride = true, u.Host
	params := component.ExporterCreateSettings{Logger: zap.NewNop(), BuildInfo: component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}}
	exp, err := f.CreateMetricsExporter(context.Background(), params, c)
	require.NoError(t, err)

	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertDouble("badattribute", math.Inf(1))
	metric := rm.InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("testmetric")

	errorFromConsumeMetrics := exp.ConsumeMetrics(ctx, md)

	err = exp.Shutdown(ctx)
	require.NoError(t, err)

	require.Error(t, errorFromConsumeMetrics)
}

func TestBadLogResourceGeneratesError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &Mock{
		Batches:    make([]Batch, 0, 1),
		StatusCode: 202,
	}

	cfg := mockConfig{useAPIKeyHeader: false}

	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	urlString := srv.URL
	if cfg.serverURL != "" {
		urlString = cfg.serverURL
	}
	u, _ := url.Parse(urlString)

	if cfg.useAPIKeyHeader {
		c.CommonConfig.APIKeyHeader = "api-key"
	} else {
		c.CommonConfig.APIKey = "NRII-1"
	}
	c.TracesConfig.insecure, c.TracesConfig.HostOverride = true, u.Host
	params := component.ExporterCreateSettings{Logger: zap.NewNop(), BuildInfo: component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}}
	exp, err := f.CreateLogsExporter(context.Background(), params, c)
	require.NoError(t, err)

	ld := pdata.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().InsertDouble("badattribute", math.Inf(1))
	rl.InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()

	errorFromConsumeLogs := exp.ConsumeLogs(ctx, ld)

	err = exp.Shutdown(ctx)
	require.NoError(t, err)

	require.Error(t, errorFromConsumeLogs)
}

func TestFailureToRecordMetricsDoesNotAffectExportingData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}
	defer view.Unregister(MetricViews()...)

	m := &Mock{
		Batches:    make([]Batch, 0, 1),
		StatusCode: 202,
	}

	cfg := mockConfig{useAPIKeyHeader: false}

	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	urlString := srv.URL
	if cfg.serverURL != "" {
		urlString = cfg.serverURL
	}
	u, _ := url.Parse(urlString)

	if cfg.useAPIKeyHeader {
		c.CommonConfig.APIKeyHeader = "api-key"
	} else {
		c.CommonConfig.APIKey = "NRII-1"
	}
	c.TracesConfig.insecure, c.TracesConfig.HostOverride = true, u.Host

	params := component.ExporterCreateSettings{Logger: zap.NewNop(), BuildInfo: component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}}
	exp, err := f.CreateTracesExporter(context.Background(), params, c)
	require.NoError(t, err)

	ptrace := pdata.NewTraces()
	s := ptrace.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	s.SetName("root")
	s.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	s.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))

	// Create a long string so that the user-agent will be too long and cause RecordMetric to fail
	b := make([]byte, 300)
	for i := 0; i < 300; i++ {
		b[i] = 'a'
	}
	consumeCtx := metadata.NewIncomingContext(context.Background(), metadata.MD{"user-agent": []string{string(b)}})
	err = exp.ConsumeTraces(consumeCtx, ptrace)
	require.NoError(t, err)
	err = exp.Shutdown(ctx)
	require.NoError(t, err)

	assert.Contains(t, m.Header[http.CanonicalHeaderKey("user-agent")][0], testCollectorName)
}
