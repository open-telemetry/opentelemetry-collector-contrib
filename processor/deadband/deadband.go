// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deadband // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deadband"

import (
	"container/list"
	"context"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deadband/internal/metadata"
)

const defaultRepeatAttribute = "telemetry.deadband.repeated"

type seriesState struct {
	name       string // owning metric name, used by shutdown flush
	lastSent   float64
	isInt      bool
	lastSentAt time.Time
	element    *list.Element // front = most recently used
}

type deadbandProcessor struct {
	cfg    *Config
	next   consumer.Metrics
	tel    *metadata.TelemetryBuilder
	logger *zap.Logger

	mu      sync.Mutex
	state   map[string]*seriesState
	lru     *list.List
	flushed bool

	boost      boostClient
	boostState bool // last observed boost-window state, for transition logging

	observedNow func() time.Time
}

func newDeadbandProcessor(
	_ context.Context,
	cfg *Config,
	set processor.Settings,
	next consumer.Metrics,
) (processor.Metrics, error) {
	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return &deadbandProcessor{
		cfg:         cfg,
		next:        next,
		tel:         tb,
		logger:      set.Logger,
		state:       make(map[string]*seriesState, 16),
		lru:         list.New(),
		observedNow: time.Now,
	}, nil
}

func (p *deadbandProcessor) Start(ctx context.Context, host component.Host) error {
	if !p.cfg.Adaptive.Enabled {
		return nil
	}
	exts := host.GetExtensions()
	if exts == nil {
		return nil
	}
	ext, ok := exts[*p.cfg.Adaptive.SignalstateExtension]
	if !ok || ext == nil {
		p.logger.Warn("deadband: adaptive extension not found; boost behavior disabled (calm state always)",
			zap.String("extension_id", p.cfg.Adaptive.SignalstateExtension.String()))
		return nil
	}
	if bc, ok := ext.(boostClient); ok {
		p.boost = bc
	} else {
		p.logger.Warn("deadband: adaptive extension does not implement the boost interface; boost disabled",
			zap.String("extension_id", p.cfg.Adaptive.SignalstateExtension.String()))
	}
	return nil
}

// boostActive reports whether suppression is currently suspended. Failure
// anywhere reads as calm state (fail toward visibility).
func (p *deadbandProcessor) boostActive(ctx context.Context) bool {
	if p.boost == nil {
		return false
	}
	return p.boost.IsOpen(ctx, p.cfg.Adaptive.Window)
}

// flushTracked emits every series' last-sent value, tagged, before boost
// processing begins — so the highest-value moment does not start in silence.
func (p *deadbandProcessor) flushTracked(ctx context.Context) {
	p.mu.Lock()
	any := len(p.state) > 0
	out := pmetric.NewMetrics()
	if any {
		nsm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
		nsm.Scope().SetName("otelcol/deadband")
		nsm.Scope().SetVersion("boost-flash")
		fm := nsm.Metrics().AppendEmpty()
		fm.SetName("deadband.boost_flush")
		fm.SetDescription("Restatement of all deadband-tracked series at boost-window entry.")
		fm.SetUnit("1")
		fg := fm.SetEmptyGauge()
		now := p.observedNow()
		for _, st := range p.state {
			dp := fg.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
			dp.Attributes().PutStr("metric_name", st.name)
			dp.Attributes().PutStr(p.cfg.RepeatAttribute, "boost")
			if st.isInt {
				dp.SetIntValue(int64(st.lastSent))
			} else {
				dp.SetDoubleValue(st.lastSent)
			}
		}
	}
	p.mu.Unlock()
	if !any {
		return
	}
	if err := p.next.ConsumeMetrics(ctx, out); err != nil {
		p.logger.Error("deadband: boost flush consumer rejected data", zap.Error(err))
	}
}

// recordObservation updates series state under boost (values pass through
// unsuppressed; state advances so calm-state resumes from the newest sent
// values — otherwise the first post-boost batch would burst).
func (p *deadbandProcessor) recordObservation(name string, dp pmetric.NumberDataPoint) {
	v, isInt, ok := valueAsDouble(dp)
	if !ok {
		return
	}
	key := seriesKey(name, dp, p.cfg.RepeatAttribute)
	p.mu.Lock()
	defer p.mu.Unlock()
	if st, ok := p.state[key]; ok {
		st.lastSent = v
		st.isInt = isInt
		st.lastSentAt = p.observedNow()
		p.lru.Remove(st.element)
		st.element = p.lru.PushFront(key)
		return
	}
	if p.cfg.State.MaxSeries > 0 && len(p.state) >= p.cfg.State.MaxSeries && !p.evictOldest() {
		return // over cap: not tracked; passes through anyway
	}
	p.state[key] = &seriesState{name: name, lastSent: v, isInt: isInt, lastSentAt: p.observedNow(), element: p.lru.PushFront(key)}
}

func (p *deadbandProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Shutdown flushes every tracked series once so collector stop does not
// look like a data gap (DESIGN promise 3). Best effort.
func (p *deadbandProcessor) Shutdown(ctx context.Context) error {
	out := pmetric.NewMetrics()

	p.mu.Lock()
	any := len(p.state) > 0
	if any {
		nsm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
		nsm.Scope().SetName("otelcol/deadband")
		fm := nsm.Metrics().AppendEmpty()
		fm.SetName("deadband.flush")
		fm.SetDescription("Last known values of all deadband-tracked series.")
		fm.SetUnit("1")
		fg := fm.SetEmptyGauge()
		now := p.observedNow()
		for _, st := range p.state {
			dp := fg.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
			dp.Attributes().PutStr("metric_name", st.name)
			if st.isInt {
				dp.SetIntValue(int64(st.lastSent))
			} else {
				dp.SetDoubleValue(st.lastSent)
			}
		}
		p.state = make(map[string]*seriesState)
		p.lru.Init()
	}
	p.mu.Unlock()

	if !any {
		return nil
	}
	if err := p.next.ConsumeMetrics(ctx, out); err != nil {
		p.logger.Error("deadband: shutdown flush consumer rejected data", zap.Error(err))
	}
	return nil
}

// ConsumeMetrics deadbands gauge datapoints and forwards the surviving
// subset. Non-gauge datapoint types and non-matching metrics pass through
// untouched (DESIGN promise 6).
func (p *deadbandProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	boost := p.boostActive(ctx)
	if boost && !p.boostState {
		// calm -> boost transition: flash every previously suppressed series
		p.flushTracked(ctx)
		p.logger.Info("deadband: boost window opened, suppression suspended")
		p.boostState = true
	} else if !boost && p.boostState {
		p.logger.Info("deadband: boost window closed, suppression resumed")
		p.boostState = false
	}

	out := pmetric.NewMetrics()

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		rmAdded := false
		var nsm pmetric.ScopeMetrics

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			mts := pmetric.NewMetricSlice()
			smAdded := false

			ms := sm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				switch {
				case boost && p.cfg.Match.matches(m.Name()) && m.Type() == pmetric.MetricTypeGauge:
					// boost: pass through unsuppressed, but keep state current
					for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
						p.recordObservation(m.Name(), m.Gauge().DataPoints().At(i))
					}
					if mts.Len() == 0 {
						mts = pmetric.NewMetricSlice()
					}
					m.CopyTo(mts.AppendEmpty())
					smAdded = true
				case !p.cfg.Match.matches(m.Name()):
					m.CopyTo(mts.AppendEmpty())
					smAdded = true
				case m.Type() != pmetric.MetricTypeGauge:
					m.CopyTo(mts.AppendEmpty())
					smAdded = true
				default:
					if p.dealWithGauge(ctx, mts, m) {
						smAdded = true
					}
				}
			}

			if smAdded {
				if !rmAdded {
					nrm := out.ResourceMetrics().AppendEmpty()
					rm.Resource().CopyTo(nrm.Resource())
					rmAdded = true
				}
				nsm = out.ResourceMetrics().At(out.ResourceMetrics().Len() - 1).ScopeMetrics().AppendEmpty()
				sm.Scope().CopyTo(nsm.Scope())
				mts.CopyTo(nsm.Metrics())
			}
		}
	}

	if out.ResourceMetrics().Len() == 0 {
		return nil
	}
	return p.next.ConsumeMetrics(ctx, out)
}

// dealWithGauge evaluates deadband rules per datapoint of one gauge metric
// and appends a filtered gauge metric into dst. Returns true if any
// datapoint survived.
func (p *deadbandProcessor) dealWithGauge(ctx context.Context, dst pmetric.MetricSlice, src pmetric.Metric) bool {
	name := src.Name()
	now := p.observedNow()
	dps := src.Gauge().DataPoints()
	kept := false

	local := pmetric.NewMetricSlice()
	fm := local.AppendEmpty()
	fm.SetName(name)
	fm.SetDescription(src.Description())
	fm.SetUnit(src.Unit())
	fg := fm.SetEmptyGauge()

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		// DESIGN promise 5: datapoints with exemplars are never suppressed.
		if dp.Exemplars().Len() > 0 {
			dp.CopyTo(fg.DataPoints().AppendEmpty())
			kept = true
			continue
		}

		emit, heartbeat := p.decide(ctx, name, dp, now)
		switch {
		case heartbeat:
			// Heartbeats restate the last-sent value (set inside decide) and
			// are tagged so consumers can tell restatements from reports.
			dp.Attributes().PutStr(p.cfg.RepeatAttribute, "true")
			dp.CopyTo(fg.DataPoints().AppendEmpty())
			kept = true
		case emit:
			dp.CopyTo(fg.DataPoints().AppendEmpty())
			kept = true
		default:
			p.tel.OtelcolDeadbandSuppressed.Add(ctx, 1, otelmetric.WithAttributes(
				attribute.String("metric_name", name)))
		}
	}

	if !kept {
		return false // the local metric is discarded wholesale
	}
	fm.CopyTo(dst.AppendEmpty())
	return true
}

// decide evaluates the rule for one datapoint and updates series state. On
// heartbeat it also restates the datapoint value to the last-sent value.
func (p *deadbandProcessor) decide(ctx context.Context, name string, dp pmetric.NumberDataPoint, now time.Time) (emit bool, heartbeat bool) {
	v, isInt, ok := valueAsDouble(dp)
	if !ok {
		return true, false // NaN / unsupported: emit, fail toward visibility
	}

	key := seriesKey(name, dp, p.cfg.RepeatAttribute)

	p.mu.Lock()
	defer p.mu.Unlock()

	st, tracked := p.state[key]
	if tracked {
		p.lru.Remove(st.element)
	} else if p.cfg.State.MaxSeries > 0 && len(p.state) >= p.cfg.State.MaxSeries && !p.evictOldest() {
		// over cardinality cap: pass through unsuppressed (fail toward visibility)
		return true, false
	}

	if !tracked {
		p.state[key] = &seriesState{name: name, lastSent: v, isInt: isInt, lastSentAt: now, element: p.lru.PushFront(key)}
		p.tel.OtelcolDeadbandSeries.Record(ctx, int64(len(p.state)))
		return true, false // first observation of a series is always emitted
	}

	switch p.cfg.Mode {
	case ModePercent:
		baseline := st.lastSent
		if baseline < 0 {
			baseline = -baseline
		}
		minRef := p.cfg.MinReference
		if minRef == 0 {
			// DESIGN: min_reference defaults to the threshold itself
			minRef = p.cfg.Threshold
		}
		if baseline < minRef {
			// degenerates to absolute mode near zero: division noise dominates
			emit = abs(v-st.lastSent) >= p.cfg.Threshold
		} else {
			emit = abs(v-st.lastSent)/baseline >= p.cfg.Threshold
		}
	default:
		emit = abs(v-st.lastSent) >= p.cfg.Threshold
	}

	if emit {
		st.lastSent = v
		st.isInt = isInt
		st.lastSentAt = now
	} else if p.cfg.StaleAfter > 0 && now.Sub(st.lastSentAt) >= p.cfg.StaleAfter {
		emit = true
		heartbeat = true
		st.lastSentAt = now
		// restate to the last-sent value, never the observed one
		if st.isInt {
			dp.SetIntValue(int64(st.lastSent))
		} else {
			dp.SetDoubleValue(st.lastSent)
		}
		p.tel.OtelcolDeadbandHeartbeat.Add(ctx, 1, otelmetric.WithAttributes(
			attribute.String("metric_name", name)))
	}

	st.element = p.lru.PushFront(key)
	return emit, heartbeat
}

func (p *deadbandProcessor) evictOldest() bool {
	el := p.lru.Back()
	if el == nil {
		return false
	}
	key, _ := el.Value.(string)
	delete(p.state, key)
	p.lru.Remove(el)
	return true
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func valueAsDouble(dp pmetric.NumberDataPoint) (float64, bool, bool) {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue(), false, true
	case pmetric.NumberDataPointValueTypeInt:
		return dp.DoubleValue(), true, true
	default:
		return 0, false, false
	}
}

// seriesKey hashes metric name plus datapoint attributes, excluding the
// repeat attribute so heartbeats cannot bifurcate a series.
func seriesKey(name string, dp pmetric.NumberDataPoint, repeatAttr string) string {
	h := fnv.New64a()
	h.Write([]byte(name))
	h.Write([]byte{0x1f})
	keys := make([]string, 0, dp.Attributes().Len())
	dp.Attributes().Range(func(k string, _ pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})
	sort.Strings(keys)
	for _, k := range keys {
		if k == repeatAttr {
			continue
		}
		v, _ := dp.Attributes().Get(k)
		h.Write([]byte(k))
		h.Write([]byte{0x1d})
		h.Write([]byte(v.AsString()))
		h.Write([]byte{0x1e})
	}
	return strconv.FormatUint(h.Sum64(), 16)
}
