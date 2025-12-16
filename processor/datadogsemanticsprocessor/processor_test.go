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
	err = tp.Start(t.Context(), &nopHost{
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
		t.Context(),
		ptrace.NewTraces(),
		func(err error) {
			assert.NoError(t, err)
		})

	m.assertBatchesLen(1)
}

func assertKeyInAttributesMatchesValue(t *testing.T, attr pcommon.Map, key, expected string) {
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
								"operation.name":   "test-operation",
								"http.status_code": 200,
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
										"exception.message":    "overridden-msg",
										"exception.type":       "overridden-type",
										"exception.stacktrace": "overridden-stack",
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
								"datadog.service":          "specified-service",
								"datadog.resource":         "specified-resource",
								"datadog.name":             "specified-operation",
								"datadog.type":             "specified-type",
								"datadog.host.name":        "specified-hostname",
								"datadog.span.kind":        "specified-span-kind",
								"datadog.env":              "specified-env",
								"datadog.http_status_code": "500",
								"datadog.error":            1,
								"datadog.error.msg":        "specified-error-msg",
								"datadog.error.type":       "specified-error-type",
								"datadog.error.stack":      "specified-error-stack",
								"operation.name":           "test-operation",
								"http.status_code":         200,
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.service", "test-service")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.env", "specified-env")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.version", "overridden-version")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.host.name", "overridden-host-name")

				span := rs.ScopeSpans().At(0).Spans().At(0)
				sattr := span.Attributes()
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.name", "test-operation")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.resource", "test-resource")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.type", "web")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.span.kind", "server")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.http_status_code", "500")
				ddError, _ := sattr.Get("datadog.error")
				require.Equal(t, int64(1), ddError.Int())
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.msg", "overridden-msg")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.type", "overridden-type")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.stack", "overridden-stack")
			},
		},
		{
			name:                          "overrideIncomingDatadogFields even if override would be empty",
			overrideIncomingDatadogFields: true,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]any{
						"service.name":                "",
						"resource.name":               "",
						"deployment.environment.name": "",
						"host.name":                   "",
						"service.version":             "",
						"datadog.env":                 "specified-host-name",
						"datadog.host.name":           "specified-host-name",
						"datadog.version":             "specified-version",
					},
					Spans: []*testutil.OTLPSpan{
						{
							Events: []testutil.OTLPSpanEvent{
								{
									Timestamp: 66,
									Name:      "exception",
									Attributes: map[string]any{
										"exception.message":    "",
										"exception.type":       "",
										"exception.stacktrace": "",
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
								"datadog.service":          "specified-service",
								"datadog.resource":         "specified-resource",
								"datadog.name":             "specified-operation",
								"datadog.type":             "specified-type",
								"datadog.host.name":        "specified-hostname",
								"datadog.span.kind":        "specified-span-kind",
								"datadog.env":              "specified-env",
								"datadog.http_status_code": "500",
								"datadog.error":            1,
								"datadog.error.msg":        "specified-error-msg",
								"datadog.error.type":       "specified-error-type",
								"datadog.error.stack":      "specified-error-stack",
								"http.status_code":         200,
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.service", "otlpresourcenoservicename")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.env", "specified-env")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.version", "")
				assertKeyInAttributesMatchesValue(t, rattr, "datadog.host.name", "")

				span := rs.ScopeSpans().At(0).Spans().At(0)
				sattr := span.Attributes()
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.name", "server.request")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.resource", "")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.type", "web")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.span.kind", "server")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.http_status_code", "500")
				ddError, _ := sattr.Get("datadog.error")
				require.Equal(t, int64(1), ddError.Int())
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.msg", "")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.type", "")
				assertKeyInAttributesMatchesValue(t, sattr, "datadog.error.stack", "")
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
										"exception.message":    "overridden-msg",
										"exception.type":       "overridden-type",
										"exception.stacktrace": "overridden-stack",
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
								"datadog.resource":         "specified-resource",
								"datadog.name":             "specified-operation",
								"datadog.type":             "specified-type",
								"datadog.span.kind":        "specified-span-kind",
								"datadog.http_status_code": "500",
								"datadog.error":            1,
								"datadog.error.msg":        "specified-error-msg",
								"datadog.error.type":       "specified-error-type",
								"datadog.error.stack":      "specified-error-stack",
								"operation.name":           "test-operation",
								"http.status_code":         200,
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
		{
			name:                          "VCS attributes mapping - span level",
			overrideIncomingDatadogFields: false,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]any{
						"service.name":                "test-service",
						"deployment.environment.name": "test-env",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]any{
								"operation.name":          "test-operation",
								"vcs.ref.head.revision":   "9d59409acf479dfa0df1aa568182e43e43df8bbe28d60fcf2bc52e30068802cc",
								"vcs.repository.url.full": "https://github.com/opentelemetry/opentelemetry-collector-contrib",
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				span := out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
				sattr := span.Attributes()
				assertKeyInAttributesMatchesValue(t, sattr, "git.commit.sha", "9d59409acf479dfa0df1aa568182e43e43df8bbe28d60fcf2bc52e30068802cc")
				assertKeyInAttributesMatchesValue(t, sattr, "git.repository_url", "github.com/opentelemetry/opentelemetry-collector-contrib")
				// Verify original VCS attributes are still present
				assertKeyInAttributesMatchesValue(t, sattr, "vcs.ref.head.revision", "9d59409acf479dfa0df1aa568182e43e43df8bbe28d60fcf2bc52e30068802cc")
				assertKeyInAttributesMatchesValue(t, sattr, "vcs.repository.url.full", "https://github.com/opentelemetry/opentelemetry-collector-contrib")
			},
		},
		{
			name:                          "VCS attributes mapping - resource level",
			overrideIncomingDatadogFields: false,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]any{
						"service.name":                "test-service",
						"deployment.environment.name": "test-env",
						"vcs.ref.head.revision":       "abc123def456",
						"vcs.repository.url.full":     "https://gitlab.com/my-org/my-project",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]any{
								"operation.name": "test-operation",
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				assertKeyInAttributesMatchesValue(t, rattr, "git.commit.sha", "abc123def456")
				assertKeyInAttributesMatchesValue(t, rattr, "git.repository_url", "gitlab.com/my-org/my-project")
				// Verify original VCS attributes are still present
				assertKeyInAttributesMatchesValue(t, rattr, "vcs.ref.head.revision", "abc123def456")
				assertKeyInAttributesMatchesValue(t, rattr, "vcs.repository.url.full", "https://gitlab.com/my-org/my-project")
			},
		},
		{
			name:                          "VCS attributes mapping - both levels",
			overrideIncomingDatadogFields: false,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]any{
						"service.name":                "test-service",
						"deployment.environment.name": "test-env",
						"vcs.ref.head.revision":       "resource-level-commit",
						"vcs.repository.url.full":     "https://github.com/resource-repo",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]any{
								"operation.name":          "test-operation",
								"vcs.ref.head.revision":   "span-level-commit",
								"vcs.repository.url.full": "https://github.com/span-repo",
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				// Resource level mappings
				assertKeyInAttributesMatchesValue(t, rattr, "git.commit.sha", "resource-level-commit")
				assertKeyInAttributesMatchesValue(t, rattr, "git.repository_url", "github.com/resource-repo")

				span := rs.ScopeSpans().At(0).Spans().At(0)
				sattr := span.Attributes()
				// Span level mappings
				assertKeyInAttributesMatchesValue(t, sattr, "git.commit.sha", "span-level-commit")
				assertKeyInAttributesMatchesValue(t, sattr, "git.repository_url", "github.com/span-repo")
			},
		},
		{
			name:                          "VCS attributes mapping with override",
			overrideIncomingDatadogFields: true,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]any{
						"service.name":                "test-service",
						"deployment.environment.name": "test-env",
						"vcs.ref.head.revision":       "new-commit",
						"vcs.repository.url.full":     "https://github.com/new-repo",
						"git.commit.sha":              "old-commit",
						"git.repository_url":          "github.com/old-repo",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]any{
								"operation.name": "test-operation",
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				// Should override existing Datadog attributes with new VCS values
				assertKeyInAttributesMatchesValue(t, rattr, "git.commit.sha", "new-commit")
				assertKeyInAttributesMatchesValue(t, rattr, "git.repository_url", "github.com/new-repo")
			},
		},
		{
			name:                          "VCS attributes mapping without override",
			overrideIncomingDatadogFields: false,
			in: []testutil.OTLPResourceSpan{
				{
					LibName:    "libname",
					LibVersion: "1.2",
					Attributes: map[string]any{
						"service.name":                "test-service",
						"deployment.environment.name": "test-env",
						"vcs.ref.head.revision":       "new-commit",
						"vcs.repository.url.full":     "https://github.com/new-repo",
						"git.commit.sha":              "existing-commit",
						"git.repository_url":          "github.com/existing-repo",
					},
					Spans: []*testutil.OTLPSpan{
						{
							TraceID:  [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
							SpanID:   [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
							ParentID: [8]byte{0, 0, 0, 0, 0, 0, 0, 1},
							Kind:     ptrace.SpanKindServer,
							Attributes: map[string]any{
								"operation.name": "test-operation",
							},
						},
					},
				},
			},
			fn: func(out *ptrace.Traces) {
				rs := out.ResourceSpans().At(0)
				rattr := rs.Resource().Attributes()
				// Should preserve existing Datadog attributes when override is false
				assertKeyInAttributesMatchesValue(t, rattr, "git.commit.sha", "existing-commit")
				assertKeyInAttributesMatchesValue(t, rattr, "git.repository_url", "github.com/existing-repo")
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
		m.testConsume(t.Context(),
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

func (*nopHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (nh *nopHost) Report(event *componentstatus.Event) {
	nh.reportFunc(event)
}
