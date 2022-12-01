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

package servicegraphprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap/zaptest"
)

func TestProcessorStart(t *testing.T) {
	// Create otlp exporters.
	otlpConfig, mexp, texp := newOTLPExporters(t)

	for _, tc := range []struct {
		name            string
		exporter        component.Exporter
		metricsExporter string
		wantErrorMsg    string
	}{
		{"export to active otlp metrics exporter", mexp, "otlp", ""},
		{"unable to find configured exporter in active exporter list", mexp, "prometheus", "failed to find metrics exporter: 'prometheus'; please configure metrics_exporter from one of: [otlp]"},
		{"export to active otlp traces exporter should error", texp, "otlp", "the exporter \"otlp\" isn't a metrics exporter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			exporters := map[component.DataType]map[component.ID]component.Exporter{
				component.DataTypeMetrics: {
					otlpConfig.ID(): tc.exporter,
				},
			}
			mHost := &mockHost{
				GetExportersFunc: func() map[component.DataType]map[component.ID]component.Exporter {
					return exporters
				},
			}

			// Create servicegraph processor
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.MetricsExporter = tc.metricsExporter

			procCreationParams := componenttest.NewNopProcessorCreateSettings()
			traceProcessor, err := factory.CreateTracesProcessor(context.Background(), procCreationParams, cfg, consumertest.NewNop())
			require.NoError(t, err)

			// Test
			smp := traceProcessor.(*processor)
			err = smp.Start(context.Background(), mHost)

			// Verify
			if tc.wantErrorMsg != "" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProcessorShutdown(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	next := new(consumertest.TracesSink)
	p := newProcessor(zaptest.NewLogger(t), cfg, next)
	err := p.Shutdown(context.Background())

	// Verify
	assert.NoError(t, err)
}

func TestProcessorConsume(t *testing.T) {
	// Prepare
	cfg := &Config{
		MetricsExporter: "mock",
		Dimensions:      []string{"some-attribute", "non-existing-attribute"},
		Store: StoreConfig{
			MaxItems: 10,
			TTL:      time.Minute,
		},
	}

	mockMetricsExporter := newMockMetricsExporter(func(md pmetric.Metrics) error {
		return verifyMetrics(t, md)
	})

	processor := newProcessor(zaptest.NewLogger(t), cfg, consumertest.NewNop())

	mHost := &mockHost{
		GetExportersFunc: func() map[component.DataType]map[component.ID]component.Exporter {
			return map[component.DataType]map[component.ID]component.Exporter{
				component.DataTypeMetrics: {
					component.NewID("mock"): mockMetricsExporter,
				},
			}
		},
	}

	assert.NoError(t, processor.Start(context.Background(), mHost))

	// Test & verify
	td := sampleTraces()
	// The assertion is part of verifyMetrics func.
	assert.NoError(t, processor.ConsumeTraces(context.Background(), td))

	// Shutdown the processor
	assert.NoError(t, processor.Shutdown(context.Background()))
}

func verifyMetrics(t *testing.T, md pmetric.Metrics) error {
	assert.Equal(t, 3, md.MetricCount())

	rms := md.ResourceMetrics()
	assert.Equal(t, 1, rms.Len())

	sms := rms.At(0).ScopeMetrics()
	assert.Equal(t, 1, sms.Len())

	ms := sms.At(0).Metrics()
	assert.Equal(t, 3, ms.Len())

	mCount := ms.At(0)
	verifyCount(t, mCount)

	mDuration := ms.At(2)
	verifyDuration(t, mDuration)

	return nil
}

func verifyCount(t *testing.T, m pmetric.Metric) {
	assert.Equal(t, "traces_service_graph_request_total", m.Name())

	assert.Equal(t, pmetric.MetricTypeSum, m.Type())
	dps := m.Sum().DataPoints()
	assert.Equal(t, 1, dps.Len())

	dp := dps.At(0)
	assert.Equal(t, pmetric.NumberDataPointValueTypeInt, dp.ValueType())
	assert.Equal(t, int64(2), dp.IntValue())

	attributes := dp.Attributes()
	assert.Equal(t, 4, attributes.Len())
	verifyAttr(t, attributes, "client", "some-service")
	verifyAttr(t, attributes, "server", "some-service")
	verifyAttr(t, attributes, "connection_type", "")
	verifyAttr(t, attributes, "client_some-attribute", "val")
}

func verifyDuration(t *testing.T, m pmetric.Metric) {
	assert.Equal(t, "traces_service_graph_request_duration_seconds", m.Name())

	assert.Equal(t, pmetric.MetricTypeHistogram, m.Type())
	dps := m.Histogram().DataPoints()
	assert.Equal(t, 1, dps.Len())

	dp := dps.At(0)
	assert.Equal(t, float64(2000), dp.Sum()) // Duration: 1sec
	assert.Equal(t, uint64(2), dp.Count())
	except := pcommon.NewUInt64Slice()
	except.FromRaw([]uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0})
	assert.Equal(t, except, dp.BucketCounts())

	attributes := dp.Attributes()
	assert.Equal(t, 4, attributes.Len())
	verifyAttr(t, attributes, "client", "some-service")
	verifyAttr(t, attributes, "server", "some-service")
	verifyAttr(t, attributes, "connection_type", "")
	verifyAttr(t, attributes, "client_some-attribute", "val")
}

func verifyAttr(t *testing.T, attrs pcommon.Map, k, expected string) {
	v, ok := attrs.Get(k)
	assert.True(t, ok)
	assert.Equal(t, expected, v.AsString())
}

func sampleTraces() ptrace.Traces {
	tStart := time.Date(2022, 1, 2, 3, 4, 5, 6, time.UTC)
	tEnd := time.Date(2022, 1, 2, 3, 4, 6, 6, time.UTC)

	traces := ptrace.NewTraces()

	resourceSpans := traces.ResourceSpans().AppendEmpty()
	resourceSpans.Resource().Attributes().PutStr(semconv.AttributeServiceName, "some-service")

	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10})
	clientSpanID := pcommon.SpanID([8]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18})

	//span one error
	clientSpan := scopeSpans.Spans().AppendEmpty()
	clientSpan.SetName("client span")
	clientSpan.SetSpanID(clientSpanID)
	clientSpan.SetTraceID(traceID)
	clientSpan.SetKind(ptrace.SpanKindClient)
	clientSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	clientSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))
	clientSpan.Attributes().PutStr("some-attribute", "val") // Attribute selected as dimension for metrics
	clientSpan.Attributes().PutStr(semconv.AttributeNetPeerIP, "127.0.0.99")
	clientSpan.Attributes().PutStr(semconv.AttributeHTTPURL, "https://www.foo.bar/search?q=OpenTelemetry#SemConv")
	clientSpan.Attributes().PutStr(semconv.AttributeRPCService, "myservice.EchoService")

	serverSpan := scopeSpans.Spans().AppendEmpty()
	serverSpan.SetName("server span")
	serverSpan.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	serverSpan.SetTraceID(traceID)
	serverSpan.SetParentSpanID(clientSpanID)
	serverSpan.SetKind(ptrace.SpanKindServer)
	serverSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	serverSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))
	serverSpan.Status().SetCode(ptrace.StatusCodeError)

	traceID = [16]byte{0x02, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}
	clientSpanID = [8]byte{0x12, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}

	//span two right
	clientSpan = scopeSpans.Spans().AppendEmpty()
	clientSpan.SetName("client span")
	clientSpan.SetSpanID(clientSpanID)
	clientSpan.SetTraceID(traceID)
	clientSpan.SetKind(ptrace.SpanKindClient)
	clientSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	clientSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))
	clientSpan.Attributes().PutStr("some-attribute", "val") // Attribute selected as dimension for metrics

	serverSpan = scopeSpans.Spans().AppendEmpty()
	serverSpan.SetName("server span")
	serverSpan.SetSpanID([8]byte{0x20, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	serverSpan.SetTraceID(traceID)
	serverSpan.SetParentSpanID(clientSpanID)
	serverSpan.SetKind(ptrace.SpanKindServer)
	serverSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	serverSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))

	return traces
}

func newOTLPExporters(t *testing.T) (*otlpexporter.Config, component.MetricsExporter, component.TracesExporter) {
	otlpExpFactory := otlpexporter.NewFactory()
	otlpConfig := &otlpexporter.Config{
		ExporterSettings: config.NewExporterSettings(component.NewID("otlp")),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: "example.com:1234",
		},
	}
	expCreationParams := componenttest.NewNopExporterCreateSettings()
	mexp, err := otlpExpFactory.CreateMetricsExporter(context.Background(), expCreationParams, otlpConfig)
	require.NoError(t, err)
	texp, err := otlpExpFactory.CreateTracesExporter(context.Background(), expCreationParams, otlpConfig)
	require.NoError(t, err)
	return otlpConfig, mexp, texp
}

var _ component.Host = (*mockHost)(nil)

type mockHost struct {
	component.Host
	GetExportersFunc func() map[component.DataType]map[component.ID]component.Exporter
}

func (m *mockHost) GetExporters() map[component.DataType]map[component.ID]component.Exporter {
	if m.GetExportersFunc != nil {
		return m.GetExportersFunc()
	}
	return m.Host.GetExporters()
}

var _ component.MetricsExporter = (*mockMetricsExporter)(nil)

func newMockMetricsExporter(verifyFunc func(md pmetric.Metrics) error) component.MetricsExporter {
	return &mockMetricsExporter{verify: verifyFunc}
}

type mockMetricsExporter struct {
	verify func(md pmetric.Metrics) error
}

func (m *mockMetricsExporter) Start(context.Context, component.Host) error { return nil }

func (m *mockMetricsExporter) Shutdown(context.Context) error { return nil }

func (m *mockMetricsExporter) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }

func (m *mockMetricsExporter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	return m.verify(md)
}
