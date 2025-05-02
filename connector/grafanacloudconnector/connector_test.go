// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gotest.tools/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector/internal/metadata"
)

func TestNewConnector(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		hostIdentifiers      []string
		metricsFlushInterval *time.Duration
		expectedConfig       *Config
	}{
		{
			name:           "default config",
			expectedConfig: createDefaultConfig().(*Config),
		},
		{
			name:                 "other config",
			hostIdentifiers:      []string{"host.id", "host.name", "k8s.node.uid"},
			metricsFlushInterval: durationPtr(15 * time.Second),
			expectedConfig: &Config{
				HostIdentifiers:      []string{"host.id", "host.name", "k8s.node.uid"},
				MetricsFlushInterval: 15 * time.Second,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			if tc.hostIdentifiers != nil {
				cfg.HostIdentifiers = tc.hostIdentifiers
			}
			if tc.metricsFlushInterval != nil {
				cfg.MetricsFlushInterval = *tc.metricsFlushInterval
			}

			c, err := factory.CreateTracesToMetrics(context.Background(), connectortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			imp := c.(*connectorImp)

			assert.NilError(t, err)
			assert.Assert(t, imp != nil)
			assert.DeepEqual(t, tc.expectedConfig.HostIdentifiers, imp.config.HostIdentifiers)
			assert.DeepEqual(t, tc.expectedConfig.MetricsFlushInterval, imp.config.MetricsFlushInterval)
		})
	}
}

func TestConsumeTraces(t *testing.T) {
	factory := NewFactory()
	testCases := []struct {
		name               string
		cfg                *Config
		input              ptrace.Traces
		expectedValue      string
		expectedMetricsLen int
	}{
		{
			name: "default",
			cfg:  factory.CreateDefaultConfig().(*Config),
			input: testTraces(
				map[string]string{
					"host.id": "foo",
				},
			),
			expectedValue:      "foo",
			expectedMetricsLen: 1,
		},
		{
			name: "multiple identifiers",
			cfg: &Config{
				HostIdentifiers: []string{"k8s.node.name", "host.id"},
			},
			input: testTraces(
				map[string]string{
					"host.id":       "foo",
					"k8s.node.name": "bar",
				},
			),
			expectedValue:      "bar",
			expectedMetricsLen: 1,
		},
		{
			name: "multiple identifiers - reverse order",
			cfg: &Config{
				HostIdentifiers: []string{"host.id", "k8s.node.name"},
			},
			input: testTraces(
				map[string]string{
					"host.id":       "foo",
					"k8s.node.name": "bar",
				},
			),
			expectedValue:      "foo",
			expectedMetricsLen: 1,
		},
		{
			name: "no attrs",
			cfg: &Config{
				HostIdentifiers: []string{"host.id", "k8s.node.name"},
			},
			input: testTraces(
				map[string]string{},
			),
			expectedValue:      "",
			expectedMetricsLen: 0,
		},
		{
			name: "no identifiers",
			cfg: &Config{
				HostIdentifiers: []string{},
			},
			input: testTraces(
				map[string]string{
					"host.id":       "foo",
					"k8s.node.name": "bar",
				},
			),
			expectedValue:      "",
			expectedMetricsLen: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.MetricsFlushInterval = 50 * time.Millisecond

			sink := &consumertest.MetricsSink{}
			c, err := factory.CreateTracesToMetrics(context.Background(), connectortest.NewNopSettings(metadata.Type), tc.cfg, sink)
			assert.NilError(t, err)

			ctx := context.Background()
			assert.NilError(t, c.Start(ctx, nil))
			err = c.ConsumeTraces(ctx, tc.input)
			assert.NilError(t, err)
			assert.NilError(t, c.Shutdown(ctx))

			metrics := sink.AllMetrics()
			assert.Equal(t, tc.expectedMetricsLen, len(metrics))

			if len(metrics) > 0 {
				sm := metrics[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				assert.Equal(t, hostInfoMetric, sm.Name())

				val, ok := sm.Gauge().DataPoints().At(0).Attributes().Get(hostIdentifierAttr)
				assert.Assert(t, ok)
				assert.Equal(t, tc.expectedValue, val.AsString())
			}
		})
	}
}

func testTraces(attrs map[string]string) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	for k, v := range attrs {
		resourceSpans.Resource().Attributes().PutStr(k, v)
	}
	return traces
}

func durationPtr(t time.Duration) *time.Duration {
	return &t
}
