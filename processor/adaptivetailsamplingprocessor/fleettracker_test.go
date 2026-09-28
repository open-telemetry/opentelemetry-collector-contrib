// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadatatest"
)

var fleetTrackerID = component.MustNewID("fake_fleet_tracker")

// fakeFleetTracker implements FleetTracker plus the no-op lifecycle methods
// needed to sit in a component.Host's extension map.
type fakeFleetTracker struct {
	initialCount int
	callback     func(count int)
	cancelCount  int
}

var _ FleetTracker = (*fakeFleetTracker)(nil)

func (f *fakeFleetTracker) SubscribeMemberCount(callback func(count int)) (func(), error) {
	f.callback = callback
	initial := f.initialCount
	if initial == 0 {
		initial = 1
	}
	callback(initial)
	return func() { f.cancelCount++ }, nil
}

func (f *fakeFleetTracker) push(n int) {
	f.callback(n)
}

func (*fakeFleetTracker) Start(context.Context, component.Host) error { return nil }
func (*fakeFleetTracker) Shutdown(context.Context) error              { return nil }

// fakeHost implements component.Host, exposing a fixed set of extensions.
type fakeHost struct {
	exts map[component.ID]component.Component
}

func (h *fakeHost) GetExtensions() map[component.ID]component.Component {
	return h.exts
}

// notAFleetTracker is a component that does not implement FleetTracker, used
// to exercise the "does not implement" resolution error.
type notAFleetTracker struct{}

func (notAFleetTracker) Start(context.Context, component.Host) error { return nil }
func (notAFleetTracker) Shutdown(context.Context) error              { return nil }

// spyThroughputSampler records SetGoalThroughputPerSec calls and Start/Stop
// invocations, standing in for a real adaptive_throughput sampler so tests
// can observe the fleet-division wiring without waiting on dynsampler-go's
// internal timing.
type spyThroughputSampler struct {
	goals      []int
	startCount int
	stopCount  int
}

func (*spyThroughputSampler) GetSampleRate(string, int) int { return 1 }
func (s *spyThroughputSampler) Start() error                { s.startCount++; return nil }
func (s *spyThroughputSampler) Stop() error                 { s.stopCount++; return nil }
func (s *spyThroughputSampler) SetGoalThroughputPerSec(goalPerSec int) {
	s.goals = append(s.goals, goalPerSec)
}

func fleetThroughputConfig(fleetTrackerID *component.ID, goalThroughput int) *Config {
	return &Config{
		TraceTimeout:   time.Hour,
		DecisionDelay:  time.Hour,
		NumTraces:      10,
		FleetTrackerID: fleetTrackerID,
		Rules: []RuleConfig{
			{Name: "throughput", Sampler: SamplerConfig{
				Type:                  AdaptiveThroughput,
				GoalThroughput:        goalThroughput,
				FingerprintAttributes: []string{`resource.attributes["service.name"]`},
			}},
		},
	}
}

func TestFleetTracker_DivisionAndSetterCalled(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	cfg := fleetThroughputConfig(&fleetTrackerID, 1000)
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)

	spy := &spyThroughputSampler{}
	p.rules[0].sampler = spy

	ft := &fakeFleetTracker{initialCount: 1}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	ruleAttrs := attribute.NewSet(attribute.String("rule", "throughput"))

	ft.push(4)
	require.Len(t, spy.goals, 1)
	assert.Equal(t, 250, spy.goals[0])
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetMemberCount(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 4}},
		metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetEffectiveGoalThroughput(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 250, Attributes: ruleAttrs}},
		metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	ft.push(2)
	require.Len(t, spy.goals, 2)
	assert.Equal(t, 500, spy.goals[1])
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetMemberCount(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 2}},
		metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
}

func TestFleetTracker_Clamp(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	cfg := fleetThroughputConfig(&fleetTrackerID, 10)
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)

	spy := &spyThroughputSampler{}
	p.rules[0].sampler = spy

	ft := &fakeFleetTracker{initialCount: 1}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	ft.push(40)
	require.Len(t, spy.goals, 1)
	assert.Equal(t, 1, spy.goals[0])
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetEffectiveGoalThroughput(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 1, Attributes: attribute.NewSet(attribute.String("rule", "throughput"))}},
		metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
}

func TestFleetTracker_KeepLastGoodOnNonPositiveCount(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	cfg := fleetThroughputConfig(&fleetTrackerID, 1000)
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)

	spy := &spyThroughputSampler{}
	p.rules[0].sampler = spy

	ft := &fakeFleetTracker{initialCount: 1}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	ft.push(4)
	require.Len(t, spy.goals, 1)

	ft.push(0)
	ft.push(-1)
	require.Len(t, spy.goals, 1, "non-positive counts must not call the setter")
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetTrackerErrors(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 2}},
		metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetMemberCount(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 4}},
		metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	ft.push(2)
	require.Len(t, spy.goals, 2)
	assert.Equal(t, 500, spy.goals[1])
}

func TestFleetTracker_UnchangedCountIsNoOp(t *testing.T) {
	cfg := fleetThroughputConfig(&fleetTrackerID, 1000)
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)

	spy := &spyThroughputSampler{}
	p.rules[0].sampler = spy

	ft := &fakeFleetTracker{initialCount: 1}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	ft.push(4)
	ft.push(4)
	assert.Len(t, spy.goals, 1, "an unchanged member count must not call the setter again")
}

func TestFleetTracker_NoSamplerReset(t *testing.T) {
	cfg := fleetThroughputConfig(&fleetTrackerID, 1000)
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)

	spy := &spyThroughputSampler{}
	p.rules[0].sampler = spy

	ft := &fakeFleetTracker{initialCount: 1}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))

	ft.push(4)
	ft.push(2)
	ft.push(8)
	assert.Equal(t, 1, spy.startCount)
	assert.Equal(t, 0, spy.stopCount)
	assert.Same(t, spy, p.rules[0].sampler, "the sampler instance must never be replaced")

	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 1, spy.stopCount)
}

func TestFleetTracker_ResolutionFailuresAtStart(t *testing.T) {
	cfg := fleetThroughputConfig(&fleetTrackerID, 1000)

	t.Run("extension_not_found", func(t *testing.T) {
		p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
		require.NoError(t, err)
		host := &fakeHost{exts: map[component.ID]component.Component{}}
		err = p.Start(t.Context(), host)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		// Start already ran the sampler-start loop before failing on fleet
		// tracker resolution, so the real sampler's background goroutine
		// must still be stopped.
		require.NoError(t, p.Shutdown(t.Context()))
	})

	t.Run("extension_wrong_type", func(t *testing.T) {
		p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
		require.NoError(t, err)
		host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: notAFleetTracker{}}}
		err = p.Start(t.Context(), host)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not implement FleetTracker")
		require.NoError(t, p.Shutdown(t.Context()))
	})

	t.Run("nil_host", func(t *testing.T) {
		p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
		require.NoError(t, err)
		err = p.Start(t.Context(), nil)
		require.Error(t, err)
		require.NoError(t, p.Shutdown(t.Context()))
		assert.Contains(t, err.Error(), "host is nil")
	})
}

func TestFleetTracker_CancelOnShutdown(t *testing.T) {
	cfg := fleetThroughputConfig(&fleetTrackerID, 1000)
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)

	ft := &fakeFleetTracker{initialCount: 1}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))

	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 1, ft.cancelCount)

	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 1, ft.cancelCount, "a second Shutdown must not re-invoke cancel")
}

func TestFleetTracker_NoThroughputRules(t *testing.T) {
	cfg := &Config{
		TraceTimeout:   time.Hour,
		DecisionDelay:  time.Hour,
		NumTraces:      10,
		FleetTrackerID: &fleetTrackerID,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{
				Type:                  AdaptivePercentage,
				GoalPercentage:        10,
				FingerprintAttributes: []string{`resource.attributes["service.name"]`},
			}},
		},
	}

	core, recorded := observer.New(zap.WarnLevel)
	settings := processortest.NewNopSettings(metadata.Type)
	settings.Logger = zap.New(core)

	p, err := newProcessor(settings, cfg, &consumertest.TracesSink{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, recorded.Len(), 1)

	ft := &fakeFleetTracker{initialCount: 1}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	assert.Nil(t, ft.callback, "SubscribeMemberCount must not be called when no rule uses adaptive_throughput")
}
