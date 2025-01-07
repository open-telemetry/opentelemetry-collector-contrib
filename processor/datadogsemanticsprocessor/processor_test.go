// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor

import (
	"context"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/testutil"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
)

func newTestTracesProcessor(cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	set := processortest.NewNopSettings()
	return createTracesProcessor(
		context.Background(),
		set,
		cfg,
		next,
	)
}

type multiTest struct {
	t *testing.T

	tp processor.Traces

	nextTrace *consumertest.TracesSink

	// ddspTrace   *datadogsemanticsprocessor
}

func newMultiTest(
	t *testing.T,
	cfg component.Config,
	errFunc func(err error),
) *multiTest {
	m := &multiTest{
		t:         t,
		nextTrace: new(consumertest.TracesSink),
	}

	tp, err := newTestTracesProcessor(cfg, m.nextTrace)
	require.NoError(t, err)
	err = tp.Start(context.Background(), &nopHost{
		reportFunc: func(event *componentstatus.Event) {
			errFunc(event.Err())
		},
	})
	if errFunc == nil {
		assert.NotNil(t, tp)
		require.NoError(t, err)
	}

	m.tp = tp
	return m
}

func (m *multiTest) testConsume(
	ctx context.Context,
	traces ptrace.Traces,
	errFunc func(err error),
) {
	errs := []error{
		m.tp.ConsumeTraces(ctx, traces),
	}

	for _, err := range errs {
		if errFunc != nil {
			errFunc(err)
		}
	}
}

func (m *multiTest) assertBatchesLen(batchesLen int) {
	require.Len(m.t, m.nextTrace.AllTraces(), batchesLen)
}

func (m *multiTest) assertResourceObjectLen(batchNo int) {
	assert.Equal(m.t, 1, m.nextTrace.AllTraces()[batchNo].ResourceSpans().Len())
}

func (m *multiTest) assertResourceAttributesLen(batchNo int, attrsLen int) {
	assert.Equal(m.t, attrsLen, m.nextTrace.AllTraces()[batchNo].ResourceSpans().At(0).Resource().Attributes().Len())
}

func (m *multiTest) assertResource(batchNum int, resourceFunc func(res pcommon.Resource)) {
	rss := m.nextTrace.AllTraces()[batchNum].ResourceSpans()
	r := rss.At(0).Resource()

	if resourceFunc != nil {
		resourceFunc(r)
	}
}

func TestNewProcessor(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()

	newMultiTest(t, cfg, nil)
}

type generateResourceFunc func(res pcommon.Resource)

func TestNilBatch(t *testing.T) {
	m := newMultiTest(t, NewFactory().CreateDefaultConfig(), nil)
	m.testConsume(
		context.Background(),
		ptrace.NewTraces(),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(1)
}

func TestBasicTranslation(t *testing.T) {
	tests := []struct {
		name                          string
		overrideIncomingDatadogFields bool
		in                            []testutil.OTLPResourceSpan
		metaOverride                  map[string]string
		metricsOverride               map[string]float64
		fn                            func(*ptrace.Traces)
	}{
		{
			name:                          "complete test",
			overrideIncomingDatadogFields: false,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]interface{}{
						"service.name":                "test-service",
						"resource.name":               "test-resource",
						"deployment.environment.name": "spanenv2",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]interface{}{
								"operation.name":                "test-operation",
								semconv.AttributeHTTPStatusCode: 200,
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				span := rs.ScopeSpans().At(0).Spans().At(0)
				ddservice, _ := span.Attributes().Get("datadog.service")
				require.Equal(t, "test-service", ddservice.AsString())
				ddname, _ := span.Attributes().Get("datadog.name")
				require.Equal(t, "test-operation", ddname.AsString())
				ddresource, _ := span.Attributes().Get("datadog.resource")
				require.Equal(t, "test-resource", ddresource.AsString())
				ddTraceID, _ := span.Attributes().Get("datadog.trace_id")
				require.Equal(t, "579005069656919567", ddTraceID.AsString())
				ddSpanID, _ := span.Attributes().Get("datadog.span_id")
				require.Equal(t, "283686952306183", ddSpanID.AsString())
				ddParentID, _ := span.Attributes().Get("datadog.parent_id")
				require.Equal(t, "1", ddParentID.AsString())
				ddType, _ := span.Attributes().Get("datadog.type")
				require.Equal(t, "web", ddType.AsString())
				meta, _ := span.Attributes().Get("datadog.meta")
				env, _ := meta.Map().Get("env")
				require.Equal(t, "spanenv2", env.AsString())
				metrics, _ := span.Attributes().Get("datadog.metrics")
				statusCode, _ := metrics.Map().Get(semconv.AttributeHTTPStatusCode)
				require.Equal(t, 200.0, statusCode.Double())
			},
		},
		{
			name:                          "overrideIncomingDatadogFields",
			overrideIncomingDatadogFields: true,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]interface{}{
						"service.name":                "test-service",
						"resource.name":               "test-resource",
						"deployment.environment.name": "spanenv2",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]interface{}{
								"datadog.service":               "specified-service",
								"datadog.resource":              "specified-resource",
								"datadog.name":                  "specified-operation",
								"datadog.trace_id":              "123456789",
								"datadog.span_id":               "987654321",
								"datadog.parent_id":             "123456789",
								"datadog.type":                  "specified-type",
								"datadog.meta":                  map[string]interface{}{"env": "specified-env"},
								"datadog.metrics":               map[string]interface{}{semconv.AttributeHTTPStatusCode: 500},
								"operation.name":                "test-operation",
								semconv.AttributeHTTPStatusCode: 200,
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				span := rs.ScopeSpans().At(0).Spans().At(0)
				ddservice, _ := span.Attributes().Get("datadog.service")
				require.Equal(t, "test-service", ddservice.AsString())
				ddname, _ := span.Attributes().Get("datadog.name")
				require.Equal(t, "test-operation", ddname.AsString())
				ddresource, _ := span.Attributes().Get("datadog.resource")
				require.Equal(t, "test-resource", ddresource.AsString())
				ddTraceID, _ := span.Attributes().Get("datadog.trace_id")
				require.Equal(t, "579005069656919567", ddTraceID.AsString())
				ddSpanID, _ := span.Attributes().Get("datadog.span_id")
				require.Equal(t, "283686952306183", ddSpanID.AsString())
				ddParentID, _ := span.Attributes().Get("datadog.parent_id")
				require.Equal(t, "1", ddParentID.AsString())
				ddType, _ := span.Attributes().Get("datadog.type")
				require.Equal(t, "web", ddType.AsString())
				meta, _ := span.Attributes().Get("datadog.meta")
				env, _ := meta.Map().Get("env")
				require.Equal(t, "spanenv2", env.AsString())
				metrics, _ := span.Attributes().Get("datadog.metrics")
				statusCode, _ := metrics.Map().Get(semconv.AttributeHTTPStatusCode)
				require.Equal(t, 200.0, statusCode.Double())
			},
		},
		{
			name:                          "don't overrideIncomingDatadogFields",
			overrideIncomingDatadogFields: false,
			metaOverride:                  map[string]string{"env": "specified-env"},
			metricsOverride:               map[string]float64{semconv.AttributeHTTPStatusCode: 500.0},
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]interface{}{
						"service.name":                "test-service",
						"resource.name":               "test-resource",
						"deployment.environment.name": "spanenv2",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]interface{}{
								"datadog.service":               "specified-service",
								"datadog.resource":              "specified-resource",
								"datadog.name":                  "specified-operation",
								"datadog.trace_id":              "123456789",
								"datadog.span_id":               "987654321",
								"datadog.parent_id":             "123456789",
								"datadog.type":                  "specified-type",
								"operation.name":                "test-operation",
								semconv.AttributeHTTPStatusCode: 200,
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				span := rs.ScopeSpans().At(0).Spans().At(0)
				ddservice, _ := span.Attributes().Get("datadog.service")
				require.Equal(t, "specified-service", ddservice.AsString())
				ddname, _ := span.Attributes().Get("datadog.name")
				require.Equal(t, "specified-operation", ddname.AsString())
				ddresource, _ := span.Attributes().Get("datadog.resource")
				require.Equal(t, "specified-resource", ddresource.AsString())
				ddTraceID, _ := span.Attributes().Get("datadog.trace_id")
				require.Equal(t, "123456789", ddTraceID.AsString())
				ddSpanID, _ := span.Attributes().Get("datadog.span_id")
				require.Equal(t, "987654321", ddSpanID.AsString())
				ddParentID, _ := span.Attributes().Get("datadog.parent_id")
				require.Equal(t, "123456789", ddParentID.AsString())
				ddType, _ := span.Attributes().Get("datadog.type")
				require.Equal(t, "specified-type", ddType.AsString())
				meta, _ := span.Attributes().Get("datadog.meta")
				env, _ := meta.Map().Get("env")
				require.Equal(t, "specified-env", env.AsString())
				metrics, _ := span.Attributes().Get("datadog.metrics")
				statusCode, _ := metrics.Map().Get(semconv.AttributeHTTPStatusCode)
				require.Equal(t, 500.0, statusCode.Double())
			},
		},
	}

	for _, tt := range tests {
		m := newMultiTest(
			t,
			&Config{
				OverrideIncomingDatadogFields: tt.overrideIncomingDatadogFields,
			},
			nil,
		)

		traces := testutil.NewOTLPTracesRequest(tt.in)

		// TODO: Update testutil.NewOTLPTracesRequest to support map[string]interface{}, then update this test to
		//  specify meta and metrics along with other attributes instead of using separate fields.
		sattr := traces.Traces().ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
		if tt.metaOverride != nil {
			meta := sattr.PutEmptyMap("datadog.meta")
			for k, v := range tt.metaOverride {
				meta.PutStr(k, v)
			}
		}
		if tt.metricsOverride != nil {
			metrics := sattr.PutEmptyMap("datadog.metrics")
			for k, v := range tt.metricsOverride {
				metrics.PutDouble(k, v)
			}
		}

		m.testConsume(context.Background(),
			traces.Traces(),
			nil,
		)

		tt.fn(&m.nextTrace.AllTraces()[0])
	}
}

func TestCapabilities(t *testing.T) {
	p, err := newTestTracesProcessor(
		NewFactory().CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	assert.NoError(t, err)
	caps := p.Capabilities()
	assert.True(t, caps.MutatesData)
}

type nopHost struct {
	reportFunc func(event *componentstatus.Event)
}

func (nh *nopHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (nh *nopHost) Report(event *componentstatus.Event) {
	nh.reportFunc(event)
}
