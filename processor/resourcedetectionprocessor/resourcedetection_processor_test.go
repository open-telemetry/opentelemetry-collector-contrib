// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
)

type MockDetector struct {
	mock.Mock
}

func (p *MockDetector) Detect(_ context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	args := p.Called()
	return args.Get(0).(pcommon.Resource), "", args.Error(1)
}

func TestResourceProcessor(t *testing.T) {
	tests := []struct {
		name             string
		detectorKeys     []string
		override         bool
		sourceResource   map[string]any
		detectedResource map[string]any
		detectedError    error
		expectedResource map[string]any
		expectedNewError string
	}{
		{
			name:     "Resource is not overridden",
			override: false,
			sourceResource: map[string]any{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
			},
			detectedResource: map[string]any{
				"cloud.availability_zone": "will-be-ignored",
				"k8s.cluster.name":        "k8s-cluster",
				"host.name":               "k8s-node",
				"bool":                    true,
				"int":                     int64(100),
				"double":                  0.1,
			},
			expectedResource: map[string]any{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
				"k8s.cluster.name":        "k8s-cluster",
				"host.name":               "k8s-node",
				"bool":                    true,
				"int":                     int64(100),
				"double":                  0.1,
			},
		},
		{
			name:     "Resource is overridden",
			override: true,
			sourceResource: map[string]any{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "will-be-overridden",
			},
			detectedResource: map[string]any{
				"cloud.availability_zone": "zone-1",
				"k8s.cluster.name":        "k8s-cluster",
				"host.name":               "k8s-node",
			},
			expectedResource: map[string]any{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "zone-1",
				"k8s.cluster.name":        "k8s-cluster",
				"host.name":               "k8s-node",
			},
		},
		{
			name: "Empty detected resource",
			sourceResource: map[string]any{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
			},
			detectedResource: map[string]any{},
			expectedResource: map[string]any{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
			},
		},
		{
			name:             "Source resource is nil",
			sourceResource:   nil,
			detectedResource: map[string]any{"host.name": "node"},
			expectedResource: map[string]any{"host.name": "node"},
		},
		{
			name:             "Detected resource is nil",
			sourceResource:   map[string]any{"host.name": "node"},
			detectedResource: nil,
			expectedResource: map[string]any{"host.name": "node"},
		},
		{
			name:             "Both resources are nil",
			sourceResource:   nil,
			detectedResource: nil,
			expectedResource: map[string]any{},
		},
		{
			name: "Detection error",
			sourceResource: map[string]any{
				"type":                    "original-type",
				"original-label":          "original-value",
				"cloud.availability_zone": "original-zone",
			},
			detectedError: errors.New("err1"),
		},
		{
			name:             "Invalid detector key",
			detectorKeys:     []string{"invalid-key"},
			expectedNewError: "invalid detector key: invalid-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &factory{providers: map[component.ID]*internal.ResourceProvider{}}

			md1 := &MockDetector{}
			res := pcommon.NewResource()
			require.NoError(t, res.Attributes().FromRaw(tt.detectedResource))
			md1.On("Detect").Return(res, tt.detectedError)
			factory.resourceProviderFactory = internal.NewProviderFactory(
				map[internal.DetectorType]internal.DetectorFactory{"mock": func(processor.CreateSettings, internal.DetectorConfig) (internal.Detector, error) {
					return md1, nil
				}})

			if tt.detectorKeys == nil {
				tt.detectorKeys = []string{"mock"}
			}

			cfg := &Config{
				Override:           tt.override,
				Detectors:          tt.detectorKeys,
				HTTPClientSettings: confighttp.HTTPClientSettings{Timeout: time.Second},
			}

			// Test trace consumer
			ttn := new(consumertest.TracesSink)
			rtp, err := factory.createTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, ttn)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rtp.Capabilities().MutatesData)

			err = rtp.Start(context.Background(), componenttest.NewNopHost())

			if tt.detectedError != nil {
				require.NoError(t, err)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rtp.Shutdown(context.Background())) }()

			td := ptrace.NewTraces()
			require.NoError(t, td.ResourceSpans().AppendEmpty().Resource().Attributes().FromRaw(tt.sourceResource))

			err = rtp.ConsumeTraces(context.Background(), td)
			require.NoError(t, err)
			got := ttn.AllTraces()[0].ResourceSpans().At(0).Resource().Attributes().AsRaw()

			assert.Equal(t, tt.expectedResource, got)

			// Test metrics consumer
			tmn := new(consumertest.MetricsSink)
			rmp, err := factory.createMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, tmn)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rmp.Capabilities().MutatesData)

			err = rmp.Start(context.Background(), componenttest.NewNopHost())

			if tt.detectedError != nil {
				require.NoError(t, err)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rmp.Shutdown(context.Background())) }()

			md := pmetric.NewMetrics()
			require.NoError(t, md.ResourceMetrics().AppendEmpty().Resource().Attributes().FromRaw(tt.sourceResource))

			err = rmp.ConsumeMetrics(context.Background(), md)
			require.NoError(t, err)
			got = tmn.AllMetrics()[0].ResourceMetrics().At(0).Resource().Attributes().AsRaw()

			assert.Equal(t, tt.expectedResource, got)

			// Test logs consumer
			tln := new(consumertest.LogsSink)
			rlp, err := factory.createLogsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, tln)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rlp.Capabilities().MutatesData)

			err = rlp.Start(context.Background(), componenttest.NewNopHost())

			if tt.detectedError != nil {
				require.NoError(t, err)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rlp.Shutdown(context.Background())) }()

			ld := plog.NewLogs()
			require.NoError(t, ld.ResourceLogs().AppendEmpty().Resource().Attributes().FromRaw(tt.sourceResource))

			err = rlp.ConsumeLogs(context.Background(), ld)
			require.NoError(t, err)
			got = tln.AllLogs()[0].ResourceLogs().At(0).Resource().Attributes().AsRaw()

			assert.Equal(t, tt.expectedResource, got)
		})
	}
}

func benchmarkConsumeTraces(b *testing.B, cfg *Config) {
	factory := NewFactory()
	sink := new(consumertest.TracesSink)
	processor, _ := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		assert.NoError(b, processor.ConsumeTraces(context.Background(), ptrace.NewTraces()))
	}
}

func BenchmarkConsumeTracesDefault(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig()
	benchmarkConsumeTraces(b, cfg.(*Config))
}

func BenchmarkConsumeTracesAll(b *testing.B) {
	cfg := &Config{Override: true, Detectors: []string{env.TypeStr, gcp.TypeStr}}
	benchmarkConsumeTraces(b, cfg)
}

func benchmarkConsumeMetrics(b *testing.B, cfg *Config) {
	factory := NewFactory()
	sink := new(consumertest.MetricsSink)
	processor, _ := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		assert.NoError(b, processor.ConsumeMetrics(context.Background(), pmetric.NewMetrics()))
	}
}

func BenchmarkConsumeMetricsDefault(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig()
	benchmarkConsumeMetrics(b, cfg.(*Config))
}

func BenchmarkConsumeMetricsAll(b *testing.B) {
	cfg := &Config{Override: true, Detectors: []string{env.TypeStr, gcp.TypeStr}}
	benchmarkConsumeMetrics(b, cfg)
}

func benchmarkConsumeLogs(b *testing.B, cfg *Config) {
	factory := NewFactory()
	sink := new(consumertest.LogsSink)
	processor, _ := factory.CreateLogsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		assert.NoError(b, processor.ConsumeLogs(context.Background(), plog.NewLogs()))
	}
}

func BenchmarkConsumeLogsDefault(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig()
	benchmarkConsumeLogs(b, cfg.(*Config))
}

func BenchmarkConsumeLogsAll(b *testing.B) {
	cfg := &Config{Override: true, Detectors: []string{env.TypeStr, gcp.TypeStr}}
	benchmarkConsumeLogs(b, cfg)
}
