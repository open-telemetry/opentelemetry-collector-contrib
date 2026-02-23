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
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

type mockDetector struct {
	mock.Mock
}

func (p *mockDetector) Detect(_ context.Context) (resource pcommon.Resource, schemaURL string, err error) {
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

			md1 := &mockDetector{}
			res := pcommon.NewResource()
			require.NoError(t, res.Attributes().FromRaw(tt.detectedResource))
			md1.On("Detect").Return(res, tt.detectedError)
			factory.resourceProviderFactory = internal.NewProviderFactory(
				map[internal.DetectorType]internal.DetectorFactory{"mock": func(processor.Settings, internal.DetectorConfig) (internal.Detector, error) {
					return md1, nil
				}})

			if tt.detectorKeys == nil {
				tt.detectorKeys = []string{"mock"}
			}

			cfg := &Config{
				Override:     tt.override,
				Detectors:    tt.detectorKeys,
				ClientConfig: confighttp.ClientConfig{Timeout: time.Second},
			}

			// Test trace consumer
			ttn := new(consumertest.TracesSink)
			rtp, err := factory.createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, ttn)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rtp.Capabilities().MutatesData)

			err = rtp.Start(t.Context(), componenttest.NewNopHost())

			if tt.detectedError != nil {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rtp.Shutdown(t.Context())) }()

			td := ptrace.NewTraces()
			require.NoError(t, td.ResourceSpans().AppendEmpty().Resource().Attributes().FromRaw(tt.sourceResource))

			err = rtp.ConsumeTraces(t.Context(), td)
			require.NoError(t, err)
			got := ttn.AllTraces()[0].ResourceSpans().At(0).Resource().Attributes().AsRaw()

			assert.Equal(t, tt.expectedResource, got)

			// Test metrics consumer
			tmn := new(consumertest.MetricsSink)
			rmp, err := factory.createMetricsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, tmn)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rmp.Capabilities().MutatesData)

			err = rmp.Start(t.Context(), componenttest.NewNopHost())

			if tt.detectedError != nil {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rmp.Shutdown(t.Context())) }()

			md := pmetric.NewMetrics()
			require.NoError(t, md.ResourceMetrics().AppendEmpty().Resource().Attributes().FromRaw(tt.sourceResource))

			err = rmp.ConsumeMetrics(t.Context(), md)
			require.NoError(t, err)
			got = tmn.AllMetrics()[0].ResourceMetrics().At(0).Resource().Attributes().AsRaw()

			assert.Equal(t, tt.expectedResource, got)

			// Test logs consumer
			tln := new(consumertest.LogsSink)
			rlp, err := factory.createLogsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, tln)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rlp.Capabilities().MutatesData)

			err = rlp.Start(t.Context(), componenttest.NewNopHost())

			if tt.detectedError != nil {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rlp.Shutdown(t.Context())) }()

			ld := plog.NewLogs()
			require.NoError(t, ld.ResourceLogs().AppendEmpty().Resource().Attributes().FromRaw(tt.sourceResource))

			err = rlp.ConsumeLogs(t.Context(), ld)
			require.NoError(t, err)
			got = tln.AllLogs()[0].ResourceLogs().At(0).Resource().Attributes().AsRaw()

			assert.Equal(t, tt.expectedResource, got)

			// Test profiles consumer
			tpn := new(consumertest.ProfilesSink)
			rpp, err := factory.createProfilesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, tpn)

			if tt.expectedNewError != "" {
				assert.EqualError(t, err, tt.expectedNewError)
				return
			}

			require.NoError(t, err)
			assert.True(t, rpp.Capabilities().MutatesData)

			err = rpp.Start(t.Context(), componenttest.NewNopHost())

			if tt.detectedError != nil {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer func() { assert.NoError(t, rpp.Shutdown(t.Context())) }()

			pd := pprofile.NewProfiles()
			require.NoError(t, pd.ResourceProfiles().AppendEmpty().Resource().Attributes().FromRaw(tt.sourceResource))

			err = rpp.ConsumeProfiles(t.Context(), pd)
			require.NoError(t, err)
			got = tpn.AllProfiles()[0].ResourceProfiles().At(0).Resource().Attributes().AsRaw()

			assert.Equal(t, tt.expectedResource, got)
		})
	}
}

func TestProcessor_RefreshInterval_UpdatesResource(t *testing.T) {
	factory := &factory{providers: map[component.ID]*internal.ResourceProvider{}}

	// First detect returns res1, then res2.
	md := &mockDetector{}
	res1 := pcommon.NewResource()
	require.NoError(t, res1.Attributes().FromRaw(map[string]any{"k": "v1"}))
	res2 := pcommon.NewResource()
	require.NoError(t, res2.Attributes().FromRaw(map[string]any{"k": "v2"}))
	md.On("Detect").Return(res1, nil).Once()
	md.On("Detect").Return(res2, nil)

	// Hook detector into factory.
	factory.resourceProviderFactory = internal.NewProviderFactory(
		map[internal.DetectorType]internal.DetectorFactory{
			"mock": func(processor.Settings, internal.DetectorConfig) (internal.Detector, error) {
				return md, nil
			},
		},
	)

	cfg := &Config{
		Detectors:       []string{"mock"},
		ClientConfig:    confighttp.ClientConfig{Timeout: 500 * time.Millisecond},
		RefreshInterval: 50 * time.Millisecond, // short to trigger refresh quickly
	}

	// Create metrics processor.
	msink := new(consumertest.MetricsSink)
	mp, err := factory.createMetricsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, msink)
	require.NoError(t, err)
	require.NoError(t, mp.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { assert.NoError(t, mp.Shutdown(t.Context())) }()

	// Send one batch → should see res1.
	md1 := pmetric.NewMetrics()
	require.NoError(t, md1.ResourceMetrics().AppendEmpty().Resource().Attributes().FromRaw(map[string]any{}))
	require.NoError(t, mp.ConsumeMetrics(t.Context(), md1))

	require.Eventually(t, func() bool {
		return len(msink.AllMetrics()) > 0
	}, time.Second, 20*time.Millisecond)
	got1 := msink.AllMetrics()[0].ResourceMetrics().At(0).Resource().Attributes().AsRaw()
	assert.Equal(t, map[string]any{"k": "v1"}, got1)

	// Verify Detect was called once (initial detection).
	md.AssertNumberOfCalls(t, "Detect", 1)

	// Wait for refresh loop to trigger and update resource.
	// Use Eventually to poll until the resource actually changes.
	require.Eventually(t, func() bool {
		// Keep sending metrics and check if resource has changed
		mdTemp := pmetric.NewMetrics()
		require.NoError(t, mdTemp.ResourceMetrics().AppendEmpty().Resource().Attributes().FromRaw(map[string]any{}))
		require.NoError(t, mp.ConsumeMetrics(t.Context(), mdTemp))

		// Check the latest metrics
		allMetrics := msink.AllMetrics()
		if len(allMetrics) == 0 {
			return false
		}
		latestAttrs := allMetrics[len(allMetrics)-1].ResourceMetrics().At(0).Resource().Attributes().AsRaw()

		// Return true if we see v2 (refresh happened)
		if v, ok := latestAttrs["k"]; ok && v == "v2" {
			return true
		}
		return false
	}, 500*time.Millisecond, 20*time.Millisecond, "refresh loop did not update resource from v1 to v2")

	// Verify Detect was called at least twice (initial + at least one refresh).
	assert.GreaterOrEqual(t, len(md.Calls), 2, "Detect should have been called at least twice")

	// Send final batch to confirm resource is now res2.
	md2 := pmetric.NewMetrics()
	require.NoError(t, md2.ResourceMetrics().AppendEmpty().Resource().Attributes().FromRaw(map[string]any{}))
	require.NoError(t, mp.ConsumeMetrics(t.Context(), md2))

	require.Eventually(t, func() bool {
		allMetrics := msink.AllMetrics()
		return len(allMetrics) >= 2
	}, time.Second, 20*time.Millisecond)

	// Check the latest metric has v2
	allMetrics := msink.AllMetrics()
	got2 := allMetrics[len(allMetrics)-1].ResourceMetrics().At(0).Resource().Attributes().AsRaw()
	assert.Equal(t, map[string]any{"k": "v2"}, got2)
}

func TestProcessor_RefreshInterval_KeepsLastGoodOnFailure(t *testing.T) {
	factory := &factory{providers: map[component.ID]*internal.ResourceProvider{}}

	// Prepare resources.
	res1 := pcommon.NewResource()
	require.NoError(t, res1.Attributes().FromRaw(map[string]any{"k": "v1"}))
	res2 := pcommon.NewResource()
	require.NoError(t, res2.Attributes().FromRaw(map[string]any{"k": "v2"}))

	// Gates to coordinate 2nd (fail) and 3rd (success) detections.
	failGate := make(chan struct{})
	successGate := make(chan struct{})

	// Mock detector:
	//  1) first call -> res1 (startup)
	//  2) second call -> block until failGate is closed, then return error (refresh keeps last good)
	//  3) third call -> block until successGate is closed, then return res2 (refresh updates)
	md := &mockDetector{}
	// 1) first call -> res1 (startup)
	md.On("Detect").Return(res1, nil).Once()

	// 2) second call -> block, then fail
	md.On("Detect").
		Run(func(_ mock.Arguments) { <-failGate }).
		Return(pcommon.NewResource(), errors.New("boom")).Once()

	// 3) third call -> block, then succeed
	md.On("Detect").
		Run(func(_ mock.Arguments) { <-successGate }).
		Return(res2, nil).Once()

	// 4) any extra calls (ticker may fire again) -> return last good value
	md.On("Detect").Return(res2, nil).Maybe()

	// Wire detector into factory.
	factory.resourceProviderFactory = internal.NewProviderFactory(
		map[internal.DetectorType]internal.DetectorFactory{
			"mock": func(processor.Settings, internal.DetectorConfig) (internal.Detector, error) {
				return md, nil
			},
		},
	)

	cfg := &Config{
		Detectors:       []string{"mock"},
		ClientConfig:    confighttp.ClientConfig{Timeout: 500 * time.Millisecond},
		RefreshInterval: 25 * time.Millisecond,
	}

	// Create and start a metrics processor so we can observe the applied resource.
	msink := new(consumertest.MetricsSink)
	mp, err := factory.createMetricsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, msink)
	require.NoError(t, err)
	require.NoError(t, mp.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { assert.NoError(t, mp.Shutdown(t.Context())) }()

	// Helper to push one metrics batch and return the resource attrs of that batch.
	getAttrsAfterConsume := func() map[string]any {
		md := pmetric.NewMetrics()
		require.NoError(t, md.ResourceMetrics().AppendEmpty().Resource().Attributes().FromRaw(map[string]any{}))
		require.NoError(t, mp.ConsumeMetrics(t.Context(), md))

		// Wait until sink has one more entry and return the last one's attrs.
		var out map[string]any
		require.Eventually(t, func() bool {
			all := msink.AllMetrics()
			if len(all) == 0 {
				return false
			}
			last := all[len(all)-1]
			out = last.ResourceMetrics().At(0).Resource().Attributes().AsRaw()
			return true
		}, time.Second, 10*time.Millisecond)
		return out
	}

	// 1) After startup, first detection applied -> expect v1.
	got := getAttrsAfterConsume()
	assert.Equal(t, map[string]any{"k": "v1"}, got)

	// 2) Let the next refresh run BUT keep it blocked on failGate.
	//    While blocked, a consume should still see v1.
	//    (The refresh goroutine is waiting; state must not change.)
	time.Sleep(2 * cfg.RefreshInterval) // give the loop a chance to enter Detect and block
	got = getAttrsAfterConsume()
	assert.Equal(t, map[string]any{"k": "v1"}, got)

	// 3) Release the failure; refresh completes with error => last good (v1) must be kept.
	close(failGate)
	// Give the loop a brief moment to finish that failed refresh.
	time.Sleep(2 * cfg.RefreshInterval)
	got = getAttrsAfterConsume()
	assert.Equal(t, map[string]any{"k": "v1"}, got)

	// 4) Now allow the next refresh to succeed (return res2).
	close(successGate)
	// Give the loop a moment to complete the successful refresh.
	require.Eventually(t, func() bool {
		attrs := getAttrsAfterConsume()
		return assert.ObjectsAreEqual(map[string]any{"k": "v2"}, attrs)
	}, time.Second, 10*time.Millisecond)

	// Verify the mock saw exactly 3 Detect calls in the order we expected.
	md.AssertExpectations(t)
}

func benchmarkConsumeTraces(b *testing.B, cfg *Config) {
	factory := NewFactory()
	sink := new(consumertest.TracesSink)
	processor, _ := factory.CreateTraces(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, sink)

	for b.Loop() {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		assert.NoError(b, processor.ConsumeTraces(b.Context(), ptrace.NewTraces()))
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
	processor, _ := factory.CreateMetrics(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, sink)

	for b.Loop() {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		assert.NoError(b, processor.ConsumeMetrics(b.Context(), pmetric.NewMetrics()))
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
	processor, _ := factory.CreateLogs(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, sink)

	for b.Loop() {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		assert.NoError(b, processor.ConsumeLogs(b.Context(), plog.NewLogs()))
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

func benchmarkConsumeProfiles(b *testing.B, cfg *Config) {
	factory := NewFactory()
	sink := new(consumertest.ProfilesSink)
	processor, _ := factory.(xprocessor.Factory).CreateProfiles(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, sink)

	for b.Loop() {
		// TODO use testbed.PerfTestDataProvider here once that includes resources
		assert.NoError(b, processor.ConsumeProfiles(b.Context(), pprofile.NewProfiles()))
	}
}

func BenchmarkConsumeProfilesDefault(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig()
	benchmarkConsumeProfiles(b, cfg.(*Config))
}

func BenchmarkConsumeProfilesAll(b *testing.B) {
	cfg := &Config{Override: true, Detectors: []string{env.TypeStr, gcp.TypeStr}}
	benchmarkConsumeProfiles(b, cfg)
}

// TestProcessorShutdownWithoutStart tests that Shutdown can be called even if Start was not called
func TestProcessorShutdownWithoutStart(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"system"}

	ctx := t.Context()
	tp, err := factory.CreateTraces(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	// Shutdown without calling Start should not panic or error
	err = tp.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestProcessorMultipleStartShutdown tests multiple Start/Shutdown cycles
func TestProcessorMultipleStartShutdown(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"env"}

	ctx := t.Context()
	host := componenttest.NewNopHost()

	mp, err := factory.CreateMetrics(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// First cycle
	err = mp.Start(ctx, host)
	require.NoError(t, err)
	err = mp.Shutdown(ctx)
	require.NoError(t, err)

	// Second cycle
	err = mp.Start(ctx, host)
	require.NoError(t, err)
	err = mp.Shutdown(ctx)
	require.NoError(t, err)

	// Third cycle
	err = mp.Start(ctx, host)
	require.NoError(t, err)
	err = mp.Shutdown(ctx)
	require.NoError(t, err)
}

// TestProcessorWithEmptyData tests processing empty telemetry data
func TestProcessorWithEmptyData(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"env"}

	ctx := t.Context()
	host := componenttest.NewNopHost()

	t.Run("empty traces", func(t *testing.T) {
		sink := new(consumertest.TracesSink)
		tp, err := factory.CreateTraces(ctx, processortest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(t, err)

		err = tp.Start(ctx, host)
		require.NoError(t, err)
		defer func() { _ = tp.Shutdown(ctx) }()

		// Process completely empty traces
		td := ptrace.NewTraces()
		err = tp.ConsumeTraces(ctx, td)
		require.NoError(t, err)
		assert.Len(t, sink.AllTraces(), 1)
	})

	t.Run("empty metrics", func(t *testing.T) {
		sink := new(consumertest.MetricsSink)
		mp, err := factory.CreateMetrics(ctx, processortest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(t, err)

		err = mp.Start(ctx, host)
		require.NoError(t, err)
		defer func() { _ = mp.Shutdown(ctx) }()

		md := pmetric.NewMetrics()
		err = mp.ConsumeMetrics(ctx, md)
		require.NoError(t, err)
		assert.Len(t, sink.AllMetrics(), 1)
	})

	t.Run("empty logs", func(t *testing.T) {
		sink := new(consumertest.LogsSink)
		lp, err := factory.CreateLogs(ctx, processortest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(t, err)

		err = lp.Start(ctx, host)
		require.NoError(t, err)
		defer func() { _ = lp.Shutdown(ctx) }()

		ld := plog.NewLogs()
		err = lp.ConsumeLogs(ctx, ld)
		require.NoError(t, err)
		assert.Len(t, sink.AllLogs(), 1)
	})

	t.Run("empty profiles", func(t *testing.T) {
		sink := new(consumertest.ProfilesSink)
		pp, err := factory.(xprocessor.Factory).CreateProfiles(ctx, processortest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(t, err)

		err = pp.Start(ctx, host)
		require.NoError(t, err)
		defer func() { _ = pp.Shutdown(ctx) }()

		pd := pprofile.NewProfiles()
		err = pp.ConsumeProfiles(ctx, pd)
		require.NoError(t, err)
		assert.Len(t, sink.AllProfiles(), 1)
	})
}

// TestProcessorWithMultipleResources tests processing data with multiple resource spans/metrics/logs
func TestProcessorWithMultipleResources(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"env"}
	oCfg.Override = false

	ctx := t.Context()
	host := componenttest.NewNopHost()

	t.Run("multiple resource spans", func(t *testing.T) {
		sink := new(consumertest.TracesSink)
		tp, err := factory.CreateTraces(ctx, processortest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(t, err)

		err = tp.Start(ctx, host)
		require.NoError(t, err)
		defer func() { _ = tp.Shutdown(ctx) }()

		td := ptrace.NewTraces()
		// Add 3 resource spans
		for range 3 {
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("original", "value")
		}

		err = tp.ConsumeTraces(ctx, td)
		require.NoError(t, err)

		result := sink.AllTraces()[0]
		assert.Equal(t, 3, result.ResourceSpans().Len())
	})

	t.Run("multiple resource metrics", func(t *testing.T) {
		sink := new(consumertest.MetricsSink)
		mp, err := factory.CreateMetrics(ctx, processortest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(t, err)

		err = mp.Start(ctx, host)
		require.NoError(t, err)
		defer func() { _ = mp.Shutdown(ctx) }()

		md := pmetric.NewMetrics()
		for range 3 {
			rm := md.ResourceMetrics().AppendEmpty()
			rm.Resource().Attributes().PutStr("original", "value")
		}

		err = mp.ConsumeMetrics(ctx, md)
		require.NoError(t, err)

		result := sink.AllMetrics()[0]
		assert.Equal(t, 3, result.ResourceMetrics().Len())
	})
}

// TestProcessorCapabilities tests that processor capabilities are correctly set
func TestProcessorCapabilities(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	ctx := t.Context()

	t.Run("traces", func(t *testing.T) {
		tp, err := factory.CreateTraces(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, tp)

		caps := tp.Capabilities()
		assert.True(t, caps.MutatesData, "processor should mutate data")
	})

	t.Run("metrics", func(t *testing.T) {
		mp, err := factory.CreateMetrics(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, mp)

		caps := mp.Capabilities()
		assert.True(t, caps.MutatesData, "processor should mutate data")
	})

	t.Run("logs", func(t *testing.T) {
		lp, err := factory.CreateLogs(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, lp)

		caps := lp.Capabilities()
		assert.True(t, caps.MutatesData, "processor should mutate data")
	})

	t.Run("profiles", func(t *testing.T) {
		pp, err := factory.(xprocessor.Factory).CreateProfiles(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, pp)

		caps := pp.Capabilities()
		assert.True(t, caps.MutatesData, "processor should mutate data")
	})
}

// TestStartFailsGracefullyOnInvalidHTTPClientConfig tests that the processor
// returns an error instead of panicking when ToClient() fails.
func TestStartFailsGracefullyOnInvalidHTTPClientConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"env"}

	// Configure invalid TLS settings that will cause ToClient() to fail
	oCfg.ClientConfig = confighttp.ClientConfig{
		TLS: configtls.ClientConfig{
			Config: configtls.Config{
				CAFile:   "/nonexistent/ca.crt",
				CertFile: "/nonexistent/cert.crt",
				KeyFile:  "/nonexistent/key.pem",
			},
		},
	}

	ctx := t.Context()
	host := componenttest.NewNopHost()

	t.Run("traces processor", func(t *testing.T) {
		tp, err := factory.CreateTraces(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, tp)

		// Before the fix, this would panic with nil pointer dereference
		// After the fix, it should return an error gracefully
		err = tp.Start(ctx, host)
		require.Error(t, err, "Start should fail when ToClient() fails")
		require.Contains(t, err.Error(), "failed to load")
	})

	t.Run("metrics processor", func(t *testing.T) {
		mp, err := factory.CreateMetrics(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, mp)

		err = mp.Start(ctx, host)
		require.Error(t, err, "Start should fail when ToClient() fails")
		require.Contains(t, err.Error(), "failed to load")
	})

	t.Run("logs processor", func(t *testing.T) {
		lp, err := factory.CreateLogs(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, lp)

		err = lp.Start(ctx, host)
		require.Error(t, err, "Start should fail when ToClient() fails")
		require.Contains(t, err.Error(), "failed to load")
	})

	t.Run("profiles processor", func(t *testing.T) {
		pp, err := factory.(xprocessor.Factory).CreateProfiles(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, pp)

		err = pp.Start(ctx, host)
		require.Error(t, err, "Start should fail when ToClient() fails")
		require.Contains(t, err.Error(), "failed to load")
	})
}
