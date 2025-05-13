// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor

import (
	"context"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/testutil"
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
	semconv "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor/internal/metadata"
)

func newTestTracesProcessor(cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	set := processortest.NewNopSettings(metadata.Type)
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

func TestNewProcessor(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()

	newMultiTest(t, cfg, nil)
}

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

func assertKeyInAttributesMatchesValue(t *testing.T, attr pcommon.Map, key string, expected string) {
	v, ok := attr.Get(key)
	require.True(t, ok)
	require.Equal(t, expected, v.AsString())
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
					Attributes: map[string]any{
						"service.name":                "test-service",
						"resource.name":               "test-resource",
						"deployment.environment.name": "spanenv2",
						"service.version":             "v2",
						"host.name":                   "test-host-name",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]any{
								"operation.name":                  "test-operation",
								string(semconv.HTTPStatusCodeKey): 200,
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.service", "test-service")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.env", "spanenv2")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.version", "v2")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.host.name", "test-host-name")

				span := rs.ScopeSpans().At(0).Spans().At(0)
				sattr := span.Attributes()
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.name", "test-operation")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.resource", "test-resource")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.type", "web")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.span.kind", "server")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.http_status_code", "200")
				ddError, _ := sattr.Get("datadog.error")
				require.Equal(t, int64(0), ddError.Int())
				_, ok := sattr.Get("datadog.error.msg")
				require.False(t, ok)
				_, ok = sattr.Get("datadog.error.type")
				require.False(t, ok)
				_, ok = sattr.Get("datadog.error.stack")
				require.False(t, ok)
			},
		},
		{
			name:                          "overrideIncomingDatadogFields",
			overrideIncomingDatadogFields: true,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]any{
						"service.name":                "test-service",
						"resource.name":               "test-resource",
						"deployment.environment.name": "spanenv2",
						"host.name":                   "overridden-host-name",
						"datadog.host.name":           "specified-host-name",
						"datadog.version":             "specified-version",
						"service.version":             "overridden-version",
					},
					Spans: []*testutil.OTLPSpan{
						{
							Events: []testutil.OTLPSpanEvent{
								{
									Timestamp: 66,
									Name:      "exception",
									Attributes: map[string]any{
										string(semconv.ExceptionMessageKey):    "overridden-msg",
										string(semconv.ExceptionTypeKey):       "overridden-type",
										string(semconv.ExceptionStacktraceKey): "overridden-stack",
									},
									Dropped: 4,
								},
							},
							StatusCode: ptrace.StatusCodeError,
							StatusMsg:  "overridden-error-msg",
							TraceID:    [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:     [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID:   [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:       ptrace.SpanKindServer,
							Attributes: map[string]any{
								"datadog.service":                 "specified-service",
								"datadog.resource":                "specified-resource",
								"datadog.name":                    "specified-operation",
								"datadog.type":                    "specified-type",
								"datadog.host.name":               "specified-hostname",
								"datadog.span.kind":               "specified-span-kind",
								"datadog.env":                     "specified-env",
								"datadog.http_status_code":        "500",
								"datadog.error":                   1,
								"datadog.error.msg":               "specified-error-msg",
								"datadog.error.type":              "specified-error-type",
								"datadog.error.stack":             "specified-error-stack",
								"operation.name":                  "test-operation",
								string(semconv.HTTPStatusCodeKey): 200,
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.service", "test-service")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.env", "spanenv2")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.version", "overridden-version")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.host.name", "overridden-host-name")

				span := rs.ScopeSpans().At(0).Spans().At(0)
				sattr := span.Attributes()
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.name", "test-operation")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.resource", "test-resource")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.type", "web")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.span.kind", "server")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.http_status_code", "200")
				ddError, _ := sattr.Get("datadog.error")
				require.Equal(t, int64(1), ddError.Int())
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.msg", "overridden-msg")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.type", "overridden-type")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.stack", "overridden-stack")
			},
		},
		{
			name:                          "dont override incoming Datadog fields",
			overrideIncomingDatadogFields: false,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]any{
						"datadog.service":             "specified-service",
						"datadog.env":                 "specified-env",
						"datadog.version":             "specified-version",
						"service.name":                "test-service",
						"resource.name":               "test-resource",
						"deployment.environment.name": "spanenv2",
						"host.name":                   "overridden-host-name",
						"datadog.host.name":           "",
					},
					Spans: []*testutil.OTLPSpan{
						{
							Events: []testutil.OTLPSpanEvent{
								{
									Timestamp: 66,
									Name:      "exception",
									Attributes: map[string]any{
										string(semconv.ExceptionMessageKey):    "overridden-msg",
										string(semconv.ExceptionTypeKey):       "overridden-type",
										string(semconv.ExceptionStacktraceKey): "overridden-stack",
									},
									Dropped: 4,
								},
							},
							StatusCode: ptrace.StatusCodeError,
							StatusMsg:  "overridden-error-msg",
							TraceID:    [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:     [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID:   [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:       ptrace.SpanKindServer,
							Attributes: map[string]any{
								"datadog.resource":                "specified-resource",
								"datadog.name":                    "specified-operation",
								"datadog.type":                    "specified-type",
								"datadog.span.kind":               "specified-span-kind",
								"datadog.http_status_code":        "500",
								"datadog.error":                   1,
								"datadog.error.msg":               "specified-error-msg",
								"datadog.error.type":              "specified-error-type",
								"datadog.error.stack":             "specified-error-stack",
								"operation.name":                  "test-operation",
								string(semconv.HTTPStatusCodeKey): 200,
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.service", "specified-service")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.env", "specified-env")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.version", "specified-version")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.host.name", "")

				span := rs.ScopeSpans().At(0).Spans().At(0)
				sattr := span.Attributes()
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.name", "specified-operation")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.resource", "specified-resource")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.type", "specified-type")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.span.kind", "specified-span-kind")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.http_status_code", "500")
				ddError, _ := sattr.Get("datadog.error")
				require.Equal(t, int64(1), ddError.Int())
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.msg", "specified-error-msg")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.type", "specified-error-type")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.stack", "specified-error-stack")
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
