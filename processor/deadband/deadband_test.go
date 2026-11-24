// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deadband

import (
	"container/list"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	componentpkg "go.opentelemetry.io/collector/component"

	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deadband/internal/metadata"
)

func gaugeMetric(name string, values ...float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rl := md.ResourceMetrics().AppendEmpty()
	sm := rl.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test")
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	m.SetUnit("1")
	g := m.SetEmptyGauge()
	now := time.Now()
	for _, v := range values {
		dp := g.DataPoints().AppendEmpty()
		dp.SetDoubleValue(v)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
	}
	return md
}

func sumMetric(name string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rl := md.ResourceMetrics().AppendEmpty()
	sm := rl.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	s := m.SetEmptySum()
	dp := s.DataPoints().AppendEmpty()
	dp.SetIntValue(7)
	return md
}

func gaugeWithExemplar(name string, value float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rl := md.ResourceMetrics().AppendEmpty()
	sm := rl.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	g := m.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(value)
	dp.Exemplars().AppendEmpty().SetTraceID([16]byte{1, 2, 3})
	return md
}

type fixture struct {
	p     *deadbandProcessor
	sink  *consumertest.MetricsSink
	now   fakeClock
	set   processor.Settings
}

type fakeClock struct{ t time.Time }

func (f *fakeClock) time() time.Time { f.t = f.t.Add(0); return f.t }
func (f *fakeClock) advance(d time.Duration) time.Time {
	f.t = f.t.Add(d)
	return f.t
}

func newTestFixture(t *testing.T, mutate func(*Config)) (*fixture, *fakeClock) {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	if mutate != nil {
		mutate(cfg)
	}
	require.NoError(t, cfg.Validate())

	fc := &fakeClock{t: time.Now()}
	sink := &consumertest.MetricsSink{}

	proc := &deadbandProcessor{
		cfg:         cfg,
		next:        sink,
		logger:      processortest.NewNopSettings(processortest.NopType).Logger,
		state:       make(map[string]*seriesState),
		lru:         list.New(),
		observedNow: fc.time,
	}
	tb, err := metadataPkg.NewTelemetryBuilder(processortest.NewNopSettings(processortest.NopType).TelemetrySettings)
	require.NoError(t, err)
	proc.tel = tb
	return &fixture{p: proc, sink: sink, now: *fc, set: processortest.NewNopSettings(processortest.NopType)}, fc
}

// --- tests -----------------------------------------------------------------

func TestFirstObservationAlwaysEmitted(t *testing.T) {
	fx, _ := newTestFixture(t, func(c *Config) {
		c.Mode = ModeAbsolute
		c.Threshold = 1.0
	})
	err := fx.p.ConsumeMetrics(context.Background(), gaugeMetric("temp", 10, 10.2, 10.4))
	require.NoError(t, err)
	out := fx.sink.AllMetrics()
	require.Len(t, out, 1)
	assert.Equal(t, 1, out[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().Len(),
		"only the first observation of a new series is emitted")
}

func TestAbsoluteDeadbandSuppressesAndEmitsOnCrossing(t *testing.T) {
	fx, _ := newTestFixture(t, func(c *Config) {
		c.Mode = ModeAbsolute
		c.Threshold = 1.0
	})

	// First batch: 10 (emitted), then 10.5 (suppressed: |delta| < 1)
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeMetric("temp", 10, 10.5)))
	// Second batch: 12 (emitted: |12-10| >= 1), 12.5 (suppressed)
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeMetric("temp", 12, 12.5)))

	emitted := collectValues(fx.sink)
	require.Equal(t, []float64{10, 12}, emitted)
	assert.Equal(t, 2, fx.sink.DataPointCount())
}

func TestPercentModeNearZeroDegradesToAbsolute(t *testing.T) {
	fx, _ := newTestFixture(t, func(c *Config) {
		c.Mode = ModePercent
		c.Threshold = 0.10 // 10%
	})

	// last = 0.05 (below MinReference = threshold = 0.10): absolute semantics.
	// 0.09 -> |0.04| < 0.10: suppressed (percent mode alone would emit 80%).
	// 0.20 -> |0.15| >= 0.10: emitted.
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeMetric("cpu.frac", 0.05, 0.09, 0.20)))
	assert.Equal(t, []float64{0.05, 0.20}, collectValues(fx.sink))
}

func TestPercentModeRelativeAboveReference(t *testing.T) {
	fx, _ := newTestFixture(t, func(c *Config) {
		c.Mode = ModePercent
		c.Threshold = 0.05 // 5%
	})

	// last = 100: 103 suppressed (3% < 5%), 106 emitted (6% >= 5%)
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeMetric("mem", 100, 103, 106)))
	assert.Equal(t, []float64{100, 106}, collectValues(fx.sink))
}

func TestHeartbeatRestatesLastSentValueAndTags(t *testing.T) {
	fx, fc := newTestFixture(t, func(c *Config) {
		c.Mode = ModeAbsolute
		c.Threshold = 100 // effectively never crosses
		c.StaleAfter = 2 * time.Minute
	})

	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeMetric("gauge", 7)))
	fc.advance(3 * time.Minute)
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeMetric("gauge", 9))) // suppressed value, but stale -> heartbeat

	vals := collectValues(fx.sink)
	require.Len(t, vals, 2)
	assert.Equal(t, float64(7), vals[1], "heartbeat restates the last-sent value, not the observed one")

	// last emitted datapoint carries the repeat attribute
	out := fx.sink.AllMetrics()
	dp := out[1].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	v, ok := dp.Attributes().Get(defaultRepeatAttribute)
	require.True(t, ok)
	assert.Equal(t, "true", v.AsString())
}

func TestNonGaugeTypesPassThrough(t *testing.T) {
	fx, _ := newTestFixture(t, func(c *Config) {
		c.Mode = ModeAbsolute
		c.Threshold = 1000
	})
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), sumMetric("counter.thing")))
	out := fx.sink.AllMetrics()
	require.Len(t, out, 1, "sums pass through untouched")
}

func TestExemplarDatapointsNeverSuppressed(t *testing.T) {
	fx, _ := newTestFixture(t, func(c *Config) {
		c.Mode = ModeAbsolute
		c.Threshold = 5 // anything smaller is suppressed
	})
	err := fx.p.ConsumeMetrics(context.Background(), gaugeMetric("temp", 1))
	require.NoError(t, err)
	// same value with exemplar: value unchanged (0 delta < 5) but must pass
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeWithExemplar("temp", 1.2)))
	vals := collectValues(fx.sink)
	require.Len(t, vals, 2, "exemplar-carrying datapoint must survive the deadband")
}

func TestStaleUnboundedRequiresNothing(t *testing.T) {
	fx, fc := newTestFixture(t, func(c *Config) {
		c.Mode = ModeAbsolute
		c.Threshold = 100
		c.StaleAfter = 0 // unbounded, explicit
	})
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeMetric("gauge", 7)))
	fc.advance(24 * time.Hour)
	require.NoError(t, fx.p.ConsumeMetrics(context.Background(), gaugeMetric("gauge", 7)))
	assert.Equal(t, 1, fx.sink.DataPointCount(), "stale_after=0 means no heartbeat")
}

const zapTestName = "deadband"

// helpers

func collectValues(sink *consumertest.MetricsSink) []float64 {
	var out []float64
	for _, md := range sink.AllMetrics() {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			for j := 0; j < rms.At(i).ScopeMetrics().Len(); j++ {
				ms := rms.At(i).ScopeMetrics().At(j).Metrics()
				for k := 0; k < ms.Len(); k++ {
					dps := ms.At(k).Gauge().DataPoints()
					for d := 0; d < dps.Len(); d++ {
						out = append(out, dps.At(d).DoubleValue())
					}
				}
			}
		}
	}
	return out
}

var _ consumer.Metrics = &consumertest.MetricsSink{}
var _ otelmetric.Int64Counter
var _ attribute.KeyValue
var _ = metadataPkg.MetricsStability

// --- X2: adaptive boost wiring ---------------------------------------------

type fakeBoost struct {
	open bool
}

func (f *fakeBoost) IsOpen(_ context.Context, _ string) bool         { return f.open }
func (f *fakeBoost) ExpiresAt(_ context.Context, _ string) time.Time { return time.Time{} }

func TestBoostSuspendsSuppressionAndFlushes(t *testing.T) {
	fc := &fakeClock{t: time.Now()}
	sink := &consumertest.MetricsSink{}
	boost := &fakeBoost{}
	cfg := createDefaultConfig().(*Config)
	cfg.Mode = ModeAbsolute
	cfg.Threshold = 1.0
	cfg.Adaptive = AdaptiveConfig{Enabled: true, Window: "boost"}
	id := componentID()
	cfg.Adaptive.SignalstateExtension = &id
	_ = cfg.Adaptive.Validate()

	tb, err := metadataPkg.NewTelemetryBuilder(processortest.NewNopSettings(processortest.NopType).TelemetrySettings)
	require.NoError(t, err)
	proc := &deadbandProcessor{
		cfg:         cfg,
		next:        sink,
		logger:      processortest.NewNopSettings(processortest.NopType).Logger,
		state:       make(map[string]*seriesState),
		lru:         list.New(),
		observedNow: fc.time,
		boost:       boost, // window opens mid-test
		tel:         tb,
	}

	// calm: 10 emitted, 10.5 suppressed
	require.NoError(t, proc.ConsumeMetrics(context.Background(), gaugeMetric("temp", 10, 10.5)))
	assert.Equal(t, []float64{10}, collectValues(sink))

	// boost window opens: 10.5 and 10.6 pass through unsuppressed AND the
	// boost-flash batch (last-sent values) precedes them
	boost.open = true
	require.NoError(t, proc.ConsumeMetrics(context.Background(), gaugeMetric("temp", 10.5, 10.6)))

	batches := sink.AllMetrics()
	require.Len(t, batches, 3, "boost entry must flush tracked series, then pass the batch through")
	fm := batches[1].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "deadband.boost_flush", fm.Name())
	fds := fm.Gauge().DataPoints()
	require.Equal(t, 1, fds.Len())
	assert.Equal(t, float64(10), fds.At(0).DoubleValue(), "boost-flash carries the last-sent value")
	v, ok := fds.At(0).Attributes().Get("telemetry.deadband.repeated")
	require.True(t, ok)
	assert.Equal(t, "boost", v.AsString())

	// state advanced under boost: calm resume with the same value stays quiet
	boost.open = false
	require.NoError(t, proc.ConsumeMetrics(context.Background(), gaugeMetric("temp", 10.6)))
	require.Len(t, sink.AllMetrics(), 3, "no burst after boost ends: state resumed current")
}

func componentID() componentpkg.ID { return componentpkg.MustNewID("signalstate") }
