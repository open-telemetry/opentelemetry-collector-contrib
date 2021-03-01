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
	"strings"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLogWriter(t *testing.T) {
	var messages []string
	l := logWriter{func(s string, _ ...zapcore.Field) {
		messages = append(messages, s)
	}}

	n, err := l.Write([]byte("one"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Len(t, messages, 1)

	n, err = l.Write([]byte("two"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Len(t, messages, 2)
}

func runMock(ptrace pdata.Traces) (*Mock, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &Mock{make([]Data, 0, 1)}
	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	c.APIKey, c.SpansURLOverride = "1", srv.URL
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
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

func testTraceData(t *testing.T, expected []Span, resource *resourcepb.Resource, spans []*tracepb.Span) {
	m, err := runMock(internaldata.OCToTraces(nil, resource, spans))
	require.NoError(t, err)
	assert.Equal(t, expected, m.Spans())
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

	_, err := runMock(ptrace)
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

	expected := []Span{
		{
			ID:      "0000000000000001",
			TraceID: "01010101010101010101010101010101",
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"name":              "root",
			},
		},
	}

	testTraceData(t, expected, nil, spans)
}

func TestExportTraceDataFullTrace(t *testing.T) {
	resource := &resourcepb.Resource{
		Labels: map[string]string{
			serviceNameKey: "test-service",
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

	expected := []Span{
		{
			ID:      "0000000000000001",
			TraceID: "01010101010101010101010101010101",
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"name":              "root",
				"resource":          "R1",
				"service.name":      "test-service",
			},
		},
		{
			ID:      "0000000000000002",
			TraceID: "01010101010101010101010101010101",
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"name":              "client",
				"parent.id":         "0000000000000001",
				"resource":          "R1",
				"service.name":      "test-service",
			},
		},
		{
			ID:      "0000000000000003",
			TraceID: "01010101010101010101010101010101",
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"name":              "server",
				"parent.id":         "0000000000000002",
				"resource":          "R1",
				"service.name":      "test-service",
			},
		},
	}

	testTraceData(t, expected, resource, spans)
}

func testExportMetricData(t *testing.T, expected []Metric, md consumerdata.MetricsData) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &Mock{make([]Data, 0, 3)}
	srv := m.Server()
	defer srv.Close()

	f := NewFactory()
	c := f.CreateDefaultConfig().(*Config)
	c.APIKey, c.MetricsURLOverride = "1", srv.URL
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exp, err := f.CreateMetricsExporter(context.Background(), params, c)
	require.NoError(t, err)
	require.NoError(t, exp.ConsumeMetrics(ctx, internaldata.OCToMetrics(md)))
	require.NoError(t, exp.Shutdown(ctx))
	assert.Equal(t, expected, m.Metrics())
}

func TestExportMetricDataMinimal(t *testing.T) {
	desc := "physical property of matter that quantitatively expresses hot and cold"
	unit := "K"
	md := consumerdata.MetricsData{
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

	expected := []Metric{
		{
			Name:      "temperature",
			Type:      "gauge",
			Value:     293.15,
			Timestamp: int64(100 * time.Microsecond),
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"description":       desc,
				"unit":              unit,
				"location":          "Portland",
				"elevation":         "0",
			},
		},
	}

	testExportMetricData(t, expected, md)
}

func TestExportMetricDataFull(t *testing.T) {
	desc := "physical property of matter that quantitatively expresses hot and cold"
	unit := "K"
	md := consumerdata.MetricsData{
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

	expected := []Metric{
		{
			Name:      "temperature",
			Type:      "gauge",
			Value:     293.15,
			Timestamp: int64(100 * time.Microsecond),
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"description":       desc,
				"unit":              unit,
				"resource":          "R1",
				"service.name":      "test-service",
				"location":          "Portland",
				"elevation":         "0",
			},
		},
		{
			Name:      "temperature",
			Type:      "gauge",
			Value:     293.15,
			Timestamp: int64(101 * time.Microsecond),
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"description":       desc,
				"unit":              unit,
				"resource":          "R1",
				"service.name":      "test-service",
				"location":          "Portland",
				"elevation":         "0",
			},
		},
		{
			Name:      "temperature",
			Type:      "gauge",
			Value:     293.45,
			Timestamp: int64(102 * time.Microsecond),
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"description":       desc,
				"unit":              unit,
				"resource":          "R1",
				"service.name":      "test-service",
				"location":          "Portland",
				"elevation":         "0",
			},
		},
		{
			Name:      "temperature",
			Type:      "gauge",
			Value:     290.05,
			Timestamp: int64(99 * time.Microsecond),
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"description":       desc,
				"unit":              unit,
				"resource":          "R1",
				"service.name":      "test-service",
				"location":          "Denver",
				"elevation":         "5280",
			},
		},
		{
			Name:      "temperature",
			Type:      "gauge",
			Value:     293.15,
			Timestamp: int64(106 * time.Microsecond),
			Attributes: map[string]interface{}{
				"collector.name":    name,
				"collector.version": version,
				"description":       desc,
				"unit":              unit,
				"resource":          "R1",
				"service.name":      "test-service",
				"location":          "Denver",
				"elevation":         "5280",
			},
		},
	}

	testExportMetricData(t, expected, md)
}
