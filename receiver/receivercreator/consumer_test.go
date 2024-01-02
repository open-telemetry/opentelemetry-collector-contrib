// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestNewEnhancingConsumer(t *testing.T) {
	podEnv, err := podEndpoint.Env()
	require.NoError(t, err)
	portEnv, err := portEndpoint.Env()
	require.NoError(t, err)
	cntrEnv, err := containerEndpoint.Env()
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	type args struct {
		resources          resourceAttributes
		resourceAttributes map[string]string
		env                observer.EndpointEnv
		endpoint           observer.Endpoint
		nextLogs           consumer.Logs
		nextMetrics        consumer.Metrics
		nextTraces         consumer.Traces
	}
	tests := []struct {
		name          string
		args          args
		want          *enhancingConsumer
		expectedError string
	}{
		{
			name: "pod endpoint",
			args: args{
				resources:   cfg.ResourceAttributes,
				env:         podEnv,
				endpoint:    podEndpoint,
				nextLogs:    &consumertest.LogsSink{},
				nextMetrics: &consumertest.MetricsSink{},
				nextTraces:  &consumertest.TracesSink{},
			},
			want: &enhancingConsumer{
				logs:    &consumertest.LogsSink{},
				metrics: &consumertest.MetricsSink{},
				traces:  &consumertest.TracesSink{},
				attrs: map[string]string{
					"k8s.pod.uid":        "uid-1",
					"k8s.pod.name":       "pod-1",
					"k8s.namespace.name": "default",
				},
			},
		},
		{
			name: "port endpoint",
			args: args{
				resources:   cfg.ResourceAttributes,
				env:         portEnv,
				endpoint:    portEndpoint,
				nextLogs:    nil,
				nextMetrics: &consumertest.MetricsSink{},
				nextTraces:  &consumertest.TracesSink{},
			},
			want: &enhancingConsumer{
				logs:    nil,
				metrics: &consumertest.MetricsSink{},
				traces:  &consumertest.TracesSink{},
				attrs: map[string]string{
					"k8s.pod.uid":        "uid-1",
					"k8s.pod.name":       "pod-1",
					"k8s.namespace.name": "default",
				},
			},
		},
		{
			name: "container endpoint",
			args: args{
				resources:   cfg.ResourceAttributes,
				env:         cntrEnv,
				endpoint:    containerEndpoint,
				nextLogs:    &consumertest.LogsSink{},
				nextMetrics: nil,
				nextTraces:  &consumertest.TracesSink{},
			},
			want: &enhancingConsumer{
				logs:    &consumertest.LogsSink{},
				metrics: nil,
				traces:  &consumertest.TracesSink{},
				attrs: map[string]string{
					"container.name":       "otel-agent",
					"container.image.name": "otelcol",
				},
			},
		},
		{
			// If the configured attribute value is empty it should not touch that
			// attribute.
			name: "attribute value empty",
			args: args{
				resources: func() resourceAttributes {
					res := createDefaultConfig().(*Config).ResourceAttributes
					res[observer.PodType]["k8s.pod.name"] = ""
					return res
				}(),
				env:         podEnv,
				endpoint:    podEndpoint,
				nextLogs:    &consumertest.LogsSink{},
				nextMetrics: &consumertest.MetricsSink{},
				nextTraces:  nil,
			},
			want: &enhancingConsumer{
				logs:    &consumertest.LogsSink{},
				metrics: &consumertest.MetricsSink{},
				traces:  nil,
				attrs: map[string]string{
					"k8s.pod.uid":        "uid-1",
					"k8s.namespace.name": "default",
				},
			},
		},
		{
			name: "both forms of resource attributes",
			args: args{
				resources: func() resourceAttributes {
					res := map[observer.EndpointType]map[string]string{observer.PodType: {}}
					for k, v := range cfg.ResourceAttributes[observer.PodType] {
						res[observer.PodType][k] = v
					}
					res[observer.PodType]["duplicate.resource.attribute"] = "pod.value"
					res[observer.PodType]["delete.me"] = "pod.value"
					return res
				}(),
				resourceAttributes: map[string]string{
					"expanded.resource.attribute":  "`'labels' in pod ? pod.labels['region'] : labels['region']`",
					"duplicate.resource.attribute": "receiver.value",
					"delete.me":                    "",
				},
				env:         podEnv,
				endpoint:    podEndpoint,
				nextLogs:    nil,
				nextMetrics: nil,
				nextTraces:  nil,
			},
			want: &enhancingConsumer{
				logs:    nil,
				metrics: nil,
				traces:  nil,
				attrs: map[string]string{
					"k8s.namespace.name":           "default",
					"k8s.pod.name":                 "pod-1",
					"k8s.pod.uid":                  "uid-1",
					"duplicate.resource.attribute": "receiver.value",
					"expanded.resource.attribute":  "west-1",
				},
			},
		},
		{
			name: "error",
			args: args{
				resources: func() resourceAttributes {
					res := createDefaultConfig().(*Config).ResourceAttributes
					res[observer.PodType]["k8s.pod.name"] = "`unbalanced"
					return res
				}(),
				env:         podEnv,
				endpoint:    podEndpoint,
				nextLogs:    &consumertest.LogsSink{},
				nextMetrics: &consumertest.MetricsSink{},
				nextTraces:  &consumertest.TracesSink{},
			},
			want:          nil,
			expectedError: `failed processing resource attribute "k8s.pod.name" for endpoint pod-1: expression was unbalanced starting at character 1`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newEnhancingConsumer(tt.args.resources, tt.args.resourceAttributes, tt.args.env, tt.args.endpoint, tt.args.nextLogs, tt.args.nextMetrics, tt.args.nextTraces)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEnhancingConsumerConsumeFunctions(t *testing.T) {
	logsFn := func() plog.Logs {
		ld := plog.NewLogs()
		ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some.log")
		return ld
	}
	expectedLogs := logsFn()

	metricsFn := func() pmetric.Metrics {
		md := pmetric.NewMetrics()
		md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("some.metric")
		return md
	}
	expectedMetrics := metricsFn()

	traceFn := func() ptrace.Traces {
		td := ptrace.NewTraces()
		td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("some.span")
		return td
	}
	expectedTraces := traceFn()

	for _, attrs := range []pcommon.Map{
		expectedLogs.ResourceLogs().At(0).Resource().Attributes(),
		expectedMetrics.ResourceMetrics().At(0).Resource().Attributes(),
		expectedTraces.ResourceSpans().At(0).Resource().Attributes(),
	} {
		attrs.PutStr("key1", "value1")
		attrs.PutStr("key2", "value2")
	}

	type consumers struct {
		nextLogs    *consumertest.LogsSink
		nextMetrics *consumertest.MetricsSink
		nextTraces  *consumertest.TracesSink
	}
	type args struct {
		ld plog.Logs
		md pmetric.Metrics
		td ptrace.Traces
	}
	type test struct {
		name            string
		consumers       consumers
		args            args
		expectedLogs    *plog.Logs
		expectedMetrics *pmetric.Metrics
		expectedTraces  *ptrace.Traces
	}

	var tests []test

	for _, _ls := range []*consumertest.LogsSink{{}, nil} {
		for _, _ms := range []*consumertest.MetricsSink{{}, nil} {
			for _, _ts := range []*consumertest.TracesSink{{}, nil} {
				ls, ms, ts := _ls, _ms, _ts
				tst := test{
					consumers: consumers{
						nextLogs:    ls,
						nextMetrics: ms,
						nextTraces:  ts,
					},
					args: args{},
				}
				if ls != nil {
					tst.name += "logs "
					tst.args.ld = logsFn()
					tst.expectedLogs = &expectedLogs
				}
				if ms != nil {
					tst.name += "metrics "
					tst.args.md = metricsFn()
					tst.expectedMetrics = &expectedMetrics
				}
				if ts != nil {
					tst.name += "traces"
					tst.args.td = traceFn()
					tst.expectedTraces = &expectedTraces
				}
				tst.name = strings.TrimSpace(tst.name)
				tests = append(tests, tst)
			}
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &enhancingConsumer{
				attrs: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			}
			if tt.consumers.nextLogs != nil {
				ec.logs = tt.consumers.nextLogs
				tt.consumers.nextLogs.Reset()
			}
			if tt.consumers.nextMetrics != nil {
				ec.metrics = tt.consumers.nextMetrics
				tt.consumers.nextMetrics.Reset()
			}
			if tt.consumers.nextTraces != nil {
				ec.traces = tt.consumers.nextTraces
				tt.consumers.nextTraces.Reset()
			}

			if tt.expectedLogs != nil {
				require.NoError(t, ec.ConsumeLogs(context.Background(), tt.args.ld))
				logs := tt.consumers.nextLogs.AllLogs()
				require.Len(t, logs, 1)
				require.NoError(t, plogtest.CompareLogs(*tt.expectedLogs, logs[0]))
			} else {
				require.EqualError(t, ec.ConsumeLogs(context.Background(), plog.NewLogs()), "no log consumer available")
			}

			if tt.expectedMetrics != nil {
				require.NoError(t, ec.ConsumeMetrics(context.Background(), tt.args.md))
				metrics := tt.consumers.nextMetrics.AllMetrics()
				require.Len(t, metrics, 1)
				require.NoError(t, pmetrictest.CompareMetrics(*tt.expectedMetrics, metrics[0]))
			} else {
				require.EqualError(t, ec.ConsumeMetrics(context.Background(), pmetric.NewMetrics()), "no metric consumer available")
			}

			if tt.expectedTraces != nil {
				require.NoError(t, ec.ConsumeTraces(context.Background(), tt.args.td))
				traces := tt.consumers.nextTraces.AllTraces()
				require.Len(t, traces, 1)
				require.NoError(t, ptracetest.CompareTraces(*tt.expectedTraces, traces[0]))
			} else {
				require.EqualError(t, ec.ConsumeTraces(context.Background(), ptrace.NewTraces()), "no trace consumer available")
			}
		})
	}
}
