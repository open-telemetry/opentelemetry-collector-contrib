// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadatatest"
)

var fleetTrackerID = component.MustNewID("fake_fleet_tracker")

// fakeFleetTracker satisfies the fleet tracker contract plus the no-op lifecycle methods
// needed to sit in a component.Host's extension map.
type fakeFleetTracker struct {
	initialCount int
	callback     func(count int)
	cancelCount  int
}

var _ fleetTracker = (*fakeFleetTracker)(nil)

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

// notAFleetTracker is a component without SubscribeMemberCount, used
// to exercise the "does not implement" resolution error.
type notAFleetTracker struct{}

func (notAFleetTracker) Start(context.Context, component.Host) error { return nil }
func (notAFleetTracker) Shutdown(context.Context) error              { return nil }

// spyThroughputSampler records SetGoalThroughputPerSec calls and Start/Stop
// invocations, standing in for a real adaptive_throughput sampler so tests
// can observe the fleet-division wiring without waiting on dynsampler-go's
// internal timing.
// Guarded by mu because the fleet_tracker contract permits concurrent
// callback delivery, which TestFleetTracker_ConcurrentCallbacks exercises.
type spyThroughputSampler struct {
	mu         sync.Mutex
	goals      []int
	startCount int
	stopCount  int
}

func (*spyThroughputSampler) GetSampleRate(string, int) int { return 1 }
func (s *spyThroughputSampler) Start() error                { s.startCount++; return nil }
func (s *spyThroughputSampler) Stop() error                 { s.stopCount++; return nil }
func (s *spyThroughputSampler) SetGoalThroughputPerSec(goalPerSec int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.goals = append(s.goals, goalPerSec)
}

// recordedGoals returns a copy of the goals seen so far.
func (s *spyThroughputSampler) recordedGoals() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.goals...)
}

// blockingThroughputSampler records its goal and then blocks inside the first
// SetGoalThroughputPerSec call, letting a test hold one fleet callback
// mid-apply while another is delivered concurrently.
type blockingThroughputSampler struct {
	spyThroughputSampler
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingThroughputSampler) SetGoalThroughputPerSec(goalPerSec int) {
	s.spyThroughputSampler.SetGoalThroughputPerSec(goalPerSec)
	s.once.Do(func() {
		close(s.entered)
		<-s.release
	})
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

	// initialCount: 4 (rather than the fake tracker's default of 1) exercises
	// the synchronous first-delivery path: the fake tracker's
	// SubscribeMemberCount invokes the callback before returning, from
	// within Start, so the goal must already be applied by the time Start
	// returns.
	ft := &fakeFleetTracker{initialCount: 4}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	require.Len(t, spy.goals, 1, "the initial fleet count must be applied synchronously before Start returns")
	assert.Equal(t, 250, spy.goals[0])
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetMemberCount(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 4}},
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
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetMemberCount(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 40}},
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

// TestFleetTracker_ConcurrentCallbacksSerialise asserts that a callback holds
// the fleet lock for its whole apply, so a second concurrent callback cannot
// interleave. The fleet_tracker contract tolerates concurrent delivery, and if
// the lock spanned only the compare and store, two callbacks could store and
// apply in opposite orders, leaving the stored count describing one goal while
// the sampler held another. That divergence is permanent, because the compare
// then skips every later apply of the stored count.
//
// The interleaving is forced rather than raced: the first callback blocks
// inside the sampler's setter, and the second must make no progress until it
// returns.
func TestFleetTracker_ConcurrentCallbacksSerialise(t *testing.T) {
	cfg := fleetThroughputConfig(&fleetTrackerID, 1000)
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)

	entered := make(chan struct{})
	release := make(chan struct{})
	spy := &blockingThroughputSampler{
		spyThroughputSampler: spyThroughputSampler{},
		entered:              entered,
		release:              release,
	}
	p.rules[0].sampler = spy

	ft := &fakeFleetTracker{initialCount: 1}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	var wg sync.WaitGroup
	wg.Go(func() {
		p.fleet.onMemberCount(2) // records 500, then blocks in the setter
	})
	<-entered

	wg.Go(func() {
		p.fleet.onMemberCount(4) // would record 250
	})

	// While the first callback is mid-apply, the second must not have applied
	// anything: it is waiting on the fleet lock.
	assert.Never(t, func() bool {
		return len(spy.recordedGoals()) > 1
	}, 100*time.Millisecond, 10*time.Millisecond,
		"a second callback applied a goal while the first was still mid-apply")

	close(release)
	wg.Wait()

	assert.Equal(t, []int{500, 250}, spy.recordedGoals())
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
		assert.Contains(t, err.Error(), "does not implement SubscribeMemberCount")
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

func TestFleetTracker_NilFleetTrackerIDNoWiring(t *testing.T) {
	// FleetTrackerID left unset must skip fleet-tracker wiring entirely: no
	// extension is resolved and the rule's sampler never receives a setter
	// call across the whole lifecycle.
	cfg := fleetThroughputConfig(nil, 1000)
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)

	spy := &spyThroughputSampler{}
	p.rules[0].sampler = spy

	require.NoError(t, p.Start(t.Context(), nil))
	require.NoError(t, p.Shutdown(t.Context()))

	assert.Empty(t, spy.goals, "no fleet_tracker wiring means SetGoalThroughputPerSec must never be called")
}

// TestFleetTracker_RealSamplerAppliesDividedGoal exercises fleet division
// against a real adaptive_throughput sampler rather than the spy used by the
// other tests in this file. The sampler wrapper's inner goal value isn't
// reachable from this package (internal/sampler exposes no accessor for it),
// so the assertions here are observable-behavior-based instead: the debug log
// emitted when a divided goal is applied through ThroughputGoalSetter, and
// that the sampler still samples/decides traces normally afterwards.
func TestFleetTracker_RealSamplerAppliesDividedGoal(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	core, recorded := observer.New(zap.DebugLevel)
	settings := metadatatest.NewSettings(tt)
	settings.Logger = zap.New(core)

	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:   time.Hour,
		DecisionDelay:  time.Millisecond,
		NumTraces:      10,
		FleetTrackerID: &fleetTrackerID,
		Rules: []RuleConfig{
			{Name: "throughput", Sampler: SamplerConfig{
				Type:                      AdaptiveThroughput,
				GoalThroughput:            1000,
				InitialSamplingPercentage: new(100.0),
				FingerprintAttributes:     []string{`resource.attributes["service.name"]`},
			}},
		},
	}
	p, err := newProcessor(settings, cfg, sink)
	require.NoError(t, err)

	ft := &fakeFleetTracker{initialCount: 4}
	host := &fakeHost{exts: map[component.ID]component.Component{fleetTrackerID: ft}}
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	// The rule's sampler does implement ThroughputGoalSetter, so the startup
	// warning (for rules whose samplers don't) must be absent here.
	assert.Empty(t, recorded.FilterLevelExact(zap.WarnLevel).All(), "no startup warning expected for a rule whose sampler implements ThroughputGoalSetter")

	// The divided goal (1000 / 4) must have been applied through the real
	// sampler's ThroughputGoalSetter, observable via the apply debug log.
	applied := recorded.FilterMessage("applied fleet-divided goal").All()
	require.Len(t, applied, 1)
	assert.Equal(t, int64(250), applied[0].ContextMap()["goal_per_sec"])
	metadatatest.AssertEqualProcessorAdaptiveTailSamplingFleetMemberCount(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 4}},
		metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())

	trace := newRootTrace(pcommon.TraceID([16]byte{0xF1}))
	trace.ResourceSpans().At(0).Resource().Attributes().PutStr("service.name", "svc")
	require.NoError(t, p.ConsumeTraces(t.Context(), trace))

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond, "the sampler must still decide traces normally after its goal is divided")
}
