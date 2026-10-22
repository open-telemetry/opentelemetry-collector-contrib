// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"context"
	"hash/maphash"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axiomhq/hyperloglog"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor/internal/metadata"
)

// staleSweepEpochs is how many consecutive zero-insert epochs a tracker must
// see before eviction. Two epochs so a tracker that fired at the tail of one
// epoch isn't reaped before traffic resumes.
const staleSweepEpochs = 2

// numShards must be a power of two so getShard reduces to a bitmask AND.
const numShards = 256

// overflowSentinel replaces the offending attribute value in
// EnforcementOverflowAttribute mode. The OTel SDK spec uses
// "otel.metric.overflow" as a boolean marker; we use a distinct value-side
// sentinel so consumers can distinguish "this dimension overflowed" from
// "the whole datapoint was marked overflow".
const overflowSentinel = "otel.cardinality_overflow"

// mustGetSketch returns a new sparse HLL++ sketch for tracker initialization
// and for replacing dense sketches during epoch rotation (p=14 → ~0.81%
// standard error).
func mustGetSketch() *hyperloglog.Sketch {
	return hyperloglog.New14()
}

// trackerKey is a zero-allocation composite key for the trackers map.
// Using a struct key avoids the heap allocation that string concatenation
// (e.g. metricName + ":" + attrKey) would cause in the hot path: Go can hash
// a struct key inline without allocating a temporary string.
type trackerKey struct {
	metricName string
	attrKey    string
}

// refreshBackoff is the least number of inserts between two estimate refreshes
// of a tracker whose growth has already been measured within reach of its
// limit (see tracker.insert). Without it a label sitting at its limit would
// re-estimate on every repeated insert. Detection lags by at most this many
// inserts, the same lag as a fixed refresh every 64 inserts.
const refreshBackoff = 64

// maxSparseSketchEntries is the largest number of distinct values guaranteed
// to leave a precision-14 HLL++ sketch sparse. The sparse buffer converts to
// dense after it exceeds 1<<14 bytes, and each entry takes at most five bytes.
// rotate reuses a sketch only when its distinct count is within this bound;
// otherwise it replaces the sketch rather than clear a potentially dense one.
const maxSparseSketchEntries = (1 << 14) / 5

// tracker holds two HLL++ sketches for a (metric_name, label_key) pair plus
// a fine-grained mutex so the shard-level lock can be released before HLL
// work begins. Hot fields are at the front for cache-line locality.
//
// current holds the values seen this epoch; window holds the union of the
// previous epoch and this one. The delta compared against the limit is
// |window| − |previous epoch|: the values that are new this epoch.
type tracker struct {
	mu      sync.Mutex
	current *hyperloglog.Sketch
	window  *hyperloglog.Sketch
	// cachedWindow is the last estimate of |window|; cachedPrevEpoch is the
	// baseline |previous epoch|, set by the first insert after a rotation.
	cachedWindow    uint64
	cachedPrevEpoch uint64
	// insertCount is inserts this epoch; sinceRefresh is inserts since
	// cachedWindow was last refreshed.
	insertCount  uint64
	sinceRefresh uint64
	// needBaseline is set by rotate so the next insert snapshots |window| as
	// the new baseline before adding to it. refreshed records whether insert
	// has refreshed cachedWindow this epoch.
	needBaseline bool
	refreshed    bool
	// idleEpochs counts consecutive zero-insert rotations; eviction at staleSweepEpochs.
	idleEpochs int
}

// insert feeds a pre-hashed value into both sketches and reports whether this
// epoch's growth exceeds limit. InsertHash is used because the []byte path
// would escape to the heap: the library's `hash` indirection blocks escape
// analysis.
//
// The cached window estimate is refreshed only when a refresh could change
// the answer. The window gains at most one distinct value per insert, so
// while cachedDelta+sinceRefresh ≤ limit the answer is "no", and once
// cachedDelta > limit it stays "yes" for the rest of the epoch: growth within
// an epoch cannot shrink. In between — a label within reach of its limit —
// insert refreshes before deciding, at most once per refreshBackoff inserts
// after the first refresh of the epoch.
func (t *tracker) insert(hashVal, limit uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.needBaseline {
		t.cachedPrevEpoch = t.window.Estimate()
		t.cachedWindow = t.cachedPrevEpoch
		t.needBaseline = false
	}
	t.current.InsertHash(hashVal)
	t.window.InsertHash(hashVal)
	t.insertCount++
	t.sinceRefresh++

	delta := t.delta()
	if delta > limit {
		return true
	}
	if delta+t.sinceRefresh <= limit {
		return false
	}
	if t.refreshed && t.sinceRefresh < refreshBackoff {
		return false
	}
	t.cachedWindow = t.window.Estimate()
	t.sinceRefresh = 0
	t.refreshed = true
	return t.delta() > limit
}

// delta is the cached growth this epoch, clamped at zero: HLL estimates of a
// dense window can read slightly below the baseline taken while it was sparse.
func (t *tracker) delta() uint64 {
	if t.cachedWindow <= t.cachedPrevEpoch {
		return 0
	}
	return t.cachedWindow - t.cachedPrevEpoch
}

// rotate promotes current → window, reuses the retired window sketch when its
// distinct count guarantees it stayed sparse, and replaces it otherwise. No
// estimate runs here: the next insert snapshots the new window as the
// baseline before adding to it. Returns true when this tracker received zero
// inserts during the epoch that just ended.
func (t *tracker) rotate() (idle bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	idle = t.insertCount == 0
	if idle {
		t.idleEpochs++
	} else {
		t.idleEpochs = 0
	}

	// The window gains at most one distinct value per insert, so this is an
	// upper bound on its entries.
	if t.cachedWindow+t.sinceRefresh <= maxSparseSketchEntries {
		t.window.Reset()
	} else {
		// Reset clears all 1<<14 dense registers; replace dense sketches to
		// avoid making that work part of every epoch rotation.
		t.window = mustGetSketch()
	}
	t.current, t.window = t.window, t.current
	// No growth to report until the next insert takes the new baseline.
	t.cachedPrevEpoch = t.cachedWindow
	t.needBaseline = true
	t.refreshed = false
	t.insertCount, t.sinceRefresh = 0, 0
	return idle
}

func newTracker() *tracker {
	return &tracker{
		current: mustGetSketch(),
		window:  mustGetSketch(),
	}
}

// offenderEntry is a snapshot of a high-delta tracker for telemetry reporting,
// produced in rotate() and consumed by the top-offenders callback.
type offenderEntry struct {
	metricName string
	labelKey   string
	delta      uint64
}

// trackerEntry is the (key, tracker) pair used to snapshot shard contents
// outside the shard lock.
type trackerEntry struct {
	key trackerKey
	t   *tracker
}

// trackerShard is an independently-locked partition of the trackers map.
// RWMutex because the common case (tracker exists) only needs a read lock.
type trackerShard struct {
	mu       sync.RWMutex
	trackers map[trackerKey]*tracker
}

type cardinalityProcessor struct {
	config          *Config
	logger          *zap.Logger
	next            consumer.Metrics
	protectedLabels map[string]struct{}
	// seed is randomized so shard distribution can't be gamed by malicious input.
	seed   maphash.Seed
	shards [numShards]*trackerShard
	// cancel stops the background rotation goroutine in Shutdown.
	cancel context.CancelFunc
	// enforcementMode mirrors config.resolvedEnforcementMode() to skip the
	// method call on every data point.
	enforcementMode EnforcementMode

	labelsStripped   atomic.Int64
	trackerCount     atomic.Int64
	trackersRejected atomic.Int64
	// rejectionWarned ensures the MaxTrackerCount warning fires only once per epoch.
	rejectionWarned atomic.Bool
	dropLogCount    atomic.Int64
	dropsThisEpoch  atomic.Int64

	// telemetry is held so Shutdown can unregister the observable callbacks;
	// without that the SDK keeps the processor (and its 256 shards) alive.
	telemetry *metadata.TelemetryBuilder

	topOffenders   []offenderEntry
	topOffendersMu sync.RWMutex

	// wg tracks the rotation goroutine so Shutdown blocks until it exits.
	wg sync.WaitGroup
}

// newCardinalityProcessor constructs a cardinalityProcessor and registers its
// internal OTel telemetry instruments. The logger is sourced from
// set.TelemetrySettings so log records carry the correct component attributes.
func newCardinalityProcessor(_ context.Context, cfg *Config, set processor.Settings, next consumer.Metrics) (processor.Metrics, error) {
	protected := make(map[string]struct{}, len(cfg.NeverDropLabels))
	for _, l := range cfg.NeverDropLabels {
		protected[l] = struct{}{}
	}

	_, cancel := context.WithCancel(context.Background())

	p := &cardinalityProcessor{
		config:          cfg,
		logger:          set.Logger,
		next:            next,
		protectedLabels: protected,
		seed:            maphash.MakeSeed(),
		cancel:          cancel,
		enforcementMode: cfg.resolvedEnforcementMode(),
	}

	for i := range p.shards {
		p.shards[i] = &trackerShard{
			trackers: make(map[trackerKey]*tracker),
		}
	}

	builder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	p.telemetry = builder

	err = builder.RegisterProcessorCardinalityTrackersActiveCallback(func(_ context.Context, o metric.Int64Observer) error {
		var totalActive int64
		for i := range p.shards {
			p.shards[i].mu.RLock()
			totalActive += int64(len(p.shards[i].trackers))
			p.shards[i].mu.RUnlock()
		}
		o.Observe(totalActive)
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = builder.RegisterProcessorCardinalityLabelsStrippedCallback(func(_ context.Context, o metric.Int64Observer) error {
		o.Observe(p.labelsStripped.Load())
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = builder.RegisterProcessorCardinalitySavingsEstimatedCallback(func(_ context.Context, o metric.Float64Observer) error {
		drops := p.labelsStripped.Load()
		o.Observe(float64(drops) * p.config.EstimatedCostPerMetricMonth)
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = builder.RegisterProcessorCardinalityTrackersRejectedCallback(func(_ context.Context, o metric.Int64Observer) error {
		o.Observe(p.trackersRejected.Load())
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = builder.RegisterProcessorCardinalityTopOffendersCallback(func(_ context.Context, o metric.Int64Observer) error {
		p.topOffendersMu.RLock()
		for _, entry := range p.topOffenders {
			o.Observe(int64(entry.delta),
				metric.WithAttributes(
					attribute.String("metric_name", entry.metricName),
					attribute.String("label_key", entry.labelKey),
				),
			)
		}
		p.topOffendersMu.RUnlock()
		return nil
	})
	if err != nil {
		return nil, err
	}

	return p, nil
}

// getShard routes a metric name to one of the numShards independent shard
// buckets. maphash.String is zero-allocation: it operates directly on the
// string header without converting to []byte, and the seed is a value type
// held on the processor struct (no pointer indirection). The seed is fixed at
// construction time so routing is deterministic within a process lifetime.
func (p *cardinalityProcessor) getShard(metricName string) *trackerShard {
	return p.shards[maphash.String(p.seed, metricName)&(numShards-1)]
}

// Capabilities tells the Collector that this processor mutates the data it
// receives. Setting MutatesData: true causes the Collector to clone the metric
// batch before passing it to this processor, ensuring that modifications to
// attributes do not race with other pipeline components that may be consuming
// the same in-memory batch concurrently.
func (*cardinalityProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeMetrics is the entry point for incoming metric batches. It walks the
// three-level OTel hierarchy (ResourceMetrics → ScopeMetrics → Metric) and
// delegates per-metric enforcement to processMetric. After all enforcement is
// complete the (potentially mutated) batch is forwarded to the next consumer.
func (p *cardinalityProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		resMetrics := rm.At(i)
		sm := resMetrics.ScopeMetrics()
		for j := 0; j < sm.Len(); j++ {
			scopeMetrics := sm.At(j)
			ms := scopeMetrics.Metrics()
			for k := 0; k < ms.Len(); k++ {
				// Process each metric
				p.processMetric(ms.At(k))
			}
		}
	}
	return p.next.ConsumeMetrics(ctx, md)
}

// processMetric dispatches a single metric to the appropriate data-point
// handler based on its type. All five OpenTelemetry metric types (Gauge, Sum,
// Histogram, ExponentialHistogram, and Summary) are fully supported, and
// attribute cardinality limits are enforced on all of their data points.
//
// When enforcement mode is strip_and_reaggregate or overflow_attribute, inline
// spatial reaggregation is performed after attribute mutation for supported
// metric types (Delta Sum and Gauge). Unsupported metric types (Cumulative Sum,
// Histogram, ExponentialHistogram, Summary) fall back to tag_only behavior.
func (p *cardinalityProcessor) processMetric(m pmetric.Metric) {
	reaggMode := p.enforcementMode == EnforcementStripAndReaggregate || p.enforcementMode == EnforcementOverflowAttribute

	switch m.Type() {
	case pmetric.MetricTypeGauge:
		p.processNumberDataPoints(m.Name(), m.Gauge().DataPoints())
		if reaggMode {
			reaggregateNumberDataPoints(m.Gauge().DataPoints(), pmetric.MetricTypeGauge, false)
		}
	case pmetric.MetricTypeSum:
		isDelta := m.Sum().AggregationTemporality() == pmetric.AggregationTemporalityDelta
		if reaggMode && !isDelta {
			// Cumulative Sums are not yet supported for reaggregation.
			// Fall back to tag_only behavior for this specific metric to avoid collisions.
			p.processNumberDataPointsWithMode(m.Name(), m.Sum().DataPoints(), EnforcementTagOnly)
		} else {
			p.processNumberDataPoints(m.Name(), m.Sum().DataPoints())
			if reaggMode && isDelta {
				reaggregateNumberDataPoints(m.Sum().DataPoints(), pmetric.MetricTypeSum, true)
			}
		}
	case pmetric.MetricTypeHistogram:
		if reaggMode {
			// Histograms are not yet supported for reaggregation.
			p.processHistogramDataPointsWithMode(m.Name(), m.Histogram().DataPoints(), EnforcementTagOnly)
		} else {
			p.processHistogramDataPoints(m.Name(), m.Histogram().DataPoints())
		}
	case pmetric.MetricTypeExponentialHistogram:
		if reaggMode {
			p.processExponentialHistogramDataPointsWithMode(m.Name(), m.ExponentialHistogram().DataPoints(), EnforcementTagOnly)
		} else {
			p.processExponentialHistogramDataPoints(m.Name(), m.ExponentialHistogram().DataPoints())
		}
	case pmetric.MetricTypeSummary:
		if reaggMode {
			p.processSummaryDataPointsWithMode(m.Name(), m.Summary().DataPoints(), EnforcementTagOnly)
		} else {
			p.processSummaryDataPoints(m.Name(), m.Summary().DataPoints())
		}
	}
}

// processNumberDataPoints iterates over a NumberDataPointSlice and calls
// handleAttributes for each data point. Both Gauge and Sum metric types use
// this slice type, so a single method covers both.
func (p *cardinalityProcessor) processNumberDataPoints(metricName string, dps pmetric.NumberDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributes(metricName, dps.At(i).Attributes())
	}
}

// processHistogramDataPoints iterates over a HistogramDataPointSlice and calls
// handleAttributes for each data point.
func (p *cardinalityProcessor) processHistogramDataPoints(metricName string, dps pmetric.HistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributes(metricName, dps.At(i).Attributes())
	}
}

// processExponentialHistogramDataPoints iterates over an ExponentialHistogramDataPointSlice
// and calls handleAttributes for each data point.
func (p *cardinalityProcessor) processExponentialHistogramDataPoints(metricName string, dps pmetric.ExponentialHistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributes(metricName, dps.At(i).Attributes())
	}
}

// processSummaryDataPoints iterates over a SummaryDataPointSlice and calls
// handleAttributes for each data point.
func (p *cardinalityProcessor) processSummaryDataPoints(metricName string, dps pmetric.SummaryDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributes(metricName, dps.At(i).Attributes())
	}
}

// handleAttributes applies the cardinality decision from shouldDrop to every
// attribute on a single data point. Tag/replace mutations are deferred until
// after RemoveIf completes — a PutBool/PutStr inside the callback could
// reallocate the pdata KeyValueList while RemoveIf still holds its internal
// cursor.
func (p *cardinalityProcessor) handleAttributes(metricName string, attrs pcommon.Map) {
	p.handleAttributesWithMode(metricName, attrs, p.enforcementMode)
}

// handleAttributesWithMode is the mode-parameterized version of handleAttributes.
// It allows callers (like processMetric) to override the enforcement mode for
// specific metric types that don't support reaggregation.
func (p *cardinalityProcessor) handleAttributesWithMode(metricName string, attrs pcommon.Map, mode EnforcementMode) {
	// shouldTag is set when tag_only mode decides an attribute should be tagged.
	// overflowKeys collects keys whose values should be replaced with the sentinel.
	// Both are deferred until after RemoveIf completes to avoid mutating the map
	// while iterating.
	shouldTag := false
	var overflowKeys []string

	attrs.RemoveIf(func(k string, v pcommon.Value) bool {
		if p.isProtected(k) {
			return false
		}

		if p.shouldDrop(metricName, k, v) {
			switch mode {
			case EnforcementTagOnly:
				// DUAL-ROUTE MODE: record the decision, keep the attribute.
				// Check() gates v.AsString() so it never runs when Debug is off.
				shouldTag = true
				if ce := p.logger.Check(zap.DebugLevel, "Cardinality overflow (tag_only)"); ce != nil {
					ce.Write(
						zap.String("metric", metricName),
						zap.String("key", k),
						zap.String("value", v.AsString()),
					)
				}
				return false

			case EnforcementOverflowAttribute:
				// OVERFLOW MODE: defer value replacement until after iteration.
				overflowKeys = append(overflowKeys, k)
				if ce := p.logger.Check(zap.DebugLevel, "Cardinality overflow (overflow_attribute)"); ce != nil {
					ce.Write(
						zap.String("metric", metricName),
						zap.String("key", k),
						zap.String("value", v.AsString()),
					)
				}
				return false

			case EnforcementStripAndReaggregate:
				// STRIP MODE: log and signal RemoveIf to delete this attribute.
				// Increment dropLogCount unconditionally so the per-epoch
				// suppression summary is accurate even when
				// DropLogMaxPerEpoch == 0 (unlimited).
				p.labelsStripped.Add(1)
				p.dropsThisEpoch.Add(1)
				count := p.dropLogCount.Add(1)
				if maxLog := p.config.DropLogMaxPerEpoch; maxLog == 0 || count <= int64(maxLog) {
					p.logger.Warn("Dropping high-cardinality attribute",
						zap.String("metric", metricName),
						zap.String("key", k),
						zap.String("value", v.AsString()))
				}
				return true
			}
		}
		return false
	})

	// Apply deferred mutations after iteration is complete.
	if shouldTag {
		attrs.PutBool("otel.metric.overflow", true)
	}
	for _, k := range overflowKeys {
		attrs.PutStr(k, overflowSentinel)
	}
}

// processNumberDataPointsWithMode processes data points with an overridden
// enforcement mode. Used when the metric type doesn't support the configured
// mode (e.g., Cumulative Sums falling back to tag_only).
func (p *cardinalityProcessor) processNumberDataPointsWithMode(metricName string, dps pmetric.NumberDataPointSlice, mode EnforcementMode) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributesWithMode(metricName, dps.At(i).Attributes(), mode)
	}
}

// processHistogramDataPointsWithMode processes histogram data points with an
// overridden enforcement mode.
func (p *cardinalityProcessor) processHistogramDataPointsWithMode(metricName string, dps pmetric.HistogramDataPointSlice, mode EnforcementMode) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributesWithMode(metricName, dps.At(i).Attributes(), mode)
	}
}

// processExponentialHistogramDataPointsWithMode processes exponential histogram
// data points with an overridden enforcement mode.
func (p *cardinalityProcessor) processExponentialHistogramDataPointsWithMode(metricName string, dps pmetric.ExponentialHistogramDataPointSlice, mode EnforcementMode) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributesWithMode(metricName, dps.At(i).Attributes(), mode)
	}
}

// processSummaryDataPointsWithMode processes summary data points with an
// overridden enforcement mode.
func (p *cardinalityProcessor) processSummaryDataPointsWithMode(metricName string, dps pmetric.SummaryDataPointSlice, mode EnforcementMode) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributesWithMode(metricName, dps.At(i).Attributes(), mode)
	}
}

// isProtected reports whether key is in the NeverDropLabels set. The lookup
// is O(1) via the pre-built map on the processor struct.
func (p *cardinalityProcessor) isProtected(key string) bool {
	_, ok := p.protectedLabels[key]
	return ok
}

// rotate advances the sliding cardinality window by one epoch across all
// shards. It runs on the background ticker goroutine, never on the hot path.
//
// Per shard: snapshot tracker pointers under a brief RLock, release, then call
// tracker.rotate on each — which resets and reuses the retired window sketch under
// the fine-grained per-tracker mutex. The shard-level write lock is never held
// during sketch reset.
func (p *cardinalityProcessor) rotate() {
	p.logger.Debug("Rotating cardinality sketches")

	// allDeltas collects the pre-rotation delta for every active tracker
	// across all shards. It is populated before rotation resets the
	// cached estimates, then sorted to extract the Top-N offenders.
	topN := p.config.TopOffendersCount
	var topBuf []offenderEntry
	if topN > 0 {
		topBuf = make([]offenderEntry, 0, topN)
	}

	for i := range p.shards {
		shard := p.shards[i]
		// Snapshot tracker references under a shard read lock — no sketch
		// allocations inside the lock.
		shard.mu.RLock()
		entries := make([]trackerEntry, len(shard.trackers))
		idx := 0
		for k, t := range shard.trackers {
			entries[idx] = trackerEntry{key: k, t: t}
			idx++
		}
		shard.mu.RUnlock()

		// Snapshot deltas before rotation collapses them to zero.
		topBuf = collectShardDeltas(entries, topBuf, topN)

		// Rotate each tracker under its own fine-grained per-tracker lock,
		// not the shard lock, so ConsumeMetrics is never blocked here.
		// Collect keys of trackers that have been idle for staleSweepEpochs
		// consecutive rotations.
		staleKeys := make([]trackerKey, 0, len(entries))
		for _, e := range entries {
			idle := e.t.rotate()
			if idle && e.t.idleEpochs >= staleSweepEpochs {
				staleKeys = append(staleKeys, e.key)
			}
		}

		// Evict stale trackers under a write lock. This is rare — only
		// trackers that received zero inserts for staleSweepEpochs
		// consecutive epochs are removed.
		if len(staleKeys) > 0 {
			shard.mu.Lock()
			for _, k := range staleKeys {
				delete(shard.trackers, k)
				p.trackerCount.Add(-1)
			}
			shard.mu.Unlock()
			p.logger.Debug("Evicted stale trackers",
				zap.Int("count", len(staleKeys)))
		}
	}

	// Reset the warning flag so we can log again next epoch if still bounded
	p.rejectionWarned.Store(false)

	// Emit suppression summary and reset drop log counters for the new epoch
	totalDrops := p.dropsThisEpoch.Swap(0)
	logged := p.dropLogCount.Swap(0)
	suppressed := totalDrops - logged
	if suppressed > 0 {
		p.logger.Info("Suppressed drop log entries this epoch",
			zap.Int64("suppressed", suppressed),
			zap.Int64("total_drops", totalDrops))
	}

	p.publishTopOffenders(topBuf)
}

// collectShardDeltas maintains a bounded top-N buffer of the highest-delta
// trackers using a linear min-scan. The buffer never grows beyond topN entries,
// so memory usage is O(topN) regardless of how many trackers exist. For each
// candidate tracker, if the buffer is not yet full the entry is appended;
// otherwise the candidate replaces the current minimum only if its delta is
// larger. The min-element index is recomputed via a simple linear scan over
// the (tiny, typically 10-element) buffer — no heap or sort allocations.
func collectShardDeltas(entries []trackerEntry, topBuf []offenderEntry, topN int) []offenderEntry {
	if topN <= 0 {
		return topBuf
	}
	for _, e := range entries {
		e.t.mu.Lock()
		if e.t.sinceRefresh > 0 {
			// insert refreshes only when the limit is within reach, so the
			// cache of a label well under its limit may be stale.
			e.t.cachedWindow = e.t.window.Estimate()
			e.t.sinceRefresh = 0
		}
		curr, prev := e.t.cachedWindow, e.t.cachedPrevEpoch
		e.t.mu.Unlock()
		if curr <= prev {
			continue
		}
		delta := curr - prev

		if len(topBuf) < topN {
			// Buffer not full yet — just append.
			topBuf = append(topBuf, offenderEntry{
				metricName: e.key.metricName,
				labelKey:   e.key.attrKey,
				delta:      delta,
			})
			continue
		}

		// Buffer is full — find the minimum element via linear scan.
		minIdx := 0
		for i := 1; i < len(topBuf); i++ {
			if topBuf[i].delta < topBuf[minIdx].delta {
				minIdx = i
			}
		}
		// Replace only if the candidate beats the current minimum.
		if delta > topBuf[minIdx].delta {
			topBuf[minIdx] = offenderEntry{
				metricName: e.key.metricName,
				labelKey:   e.key.attrKey,
				delta:      delta,
			}
		}
	}
	return topBuf
}

// publishTopOffenders sorts the bounded top-N buffer by descending delta and
// stores the result under topOffendersMu for the telemetry callback to read.
// It also emits an Info-level log line for the single highest offender to aid
// grep-based debugging. This is a no-op when the buffer is empty.
func (p *cardinalityProcessor) publishTopOffenders(topBuf []offenderEntry) {
	if len(topBuf) == 0 {
		return
	}
	// Sort the small bounded buffer (typically 10 elements) for deterministic
	// gauge emission order. This is a single sort of a tiny slice, not the
	// unbounded sort that the previous implementation used.
	sortOffenders(topBuf)
	p.topOffendersMu.Lock()
	p.topOffenders = topBuf
	p.topOffendersMu.Unlock()

	p.logger.Info("Top cardinality offender",
		zap.String("metric", topBuf[0].metricName),
		zap.String("label", topBuf[0].labelKey),
		zap.Uint64("delta", topBuf[0].delta))
}

// sortOffenders performs an insertion sort on a small offenderEntry slice in
// descending delta order. Insertion sort is optimal here because the slice is
// bounded to TopOffendersCount (default 10) — well below the crossover point
// where quicksort becomes worthwhile — and it avoids the closure allocation
// that sort.Slice would incur.
func sortOffenders(s []offenderEntry) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j].delta < key.delta {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

// Start is called by the OTel Collector host when the pipeline is starting.
// It launches the background epoch-rotation goroutine and returns immediately.
// The goroutine exits when the internal cancel function is called, which
// happens in Shutdown().
func (p *cardinalityProcessor) Start(_ context.Context, _ component.Host) error {
	// Create a local cancelable context rooted in context.Background()
	// to ensure it is not affected by the ephemeral startup context.
	childCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	p.logger.Info("Starting cardinality rotation ticker",
		zap.Int("interval_seconds", p.config.EpochDurationSeconds))

	p.wg.Go(func() {
		p.rotationLoop(childCtx)
	})

	return nil
}

// rotationLoop runs the background epoch-rotation ticker. It runs until
// the provided context is canceled.
func (p *cardinalityProcessor) rotationLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(p.config.EpochDurationSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.rotate()
		case <-ctx.Done():
			return
		}
	}
}

// Shutdown is called by the OTel Collector host when the pipeline is stopping.
// It must complete two cleanup steps in order:
//
//  1. Unregister the observable gauge callback. The OTel SDK holds a strong
//     reference to every registered callback closure; without this call the
//     closure — which captures p via the p.shards slice — would keep the entire
//     processor alive until the SDK's MeterProvider is shut down, leaking all
//     256 shard maps and their tracker contents across pipeline reconfigurations.
//     A nil guard is included for safety (e.g. if construction failed after the
//     counter registrations but before RegisterCallback).
//
//  2. Cancel the child context. This signals the background ticker goroutine to
//     exit on its next select iteration, stopping epoch rotations cleanly.
func (p *cardinalityProcessor) Shutdown(_ context.Context) error {
	p.logger.Info("Shutting down cardinality processor")
	if p.telemetry != nil {
		p.telemetry.Shutdown()
	}
	p.cancel()
	p.wg.Wait()
	return nil
}

// shouldDrop returns true when the unique-value count for (metricName, attrKey)
// has grown by more than MaxCardinalityDeltaPerEpoch since the last epoch
// rotation. The fast path is a shard RLock; a missed tracker triggers
// double-checked locking to install one. curr ≤ prev is treated as no growth
// to guard against uint64 underflow from HLL variance near sketch boundaries.
func (p *cardinalityProcessor) shouldDrop(metricName, attrKey string, attrVal pcommon.Value) bool {
	key := trackerKey{metricName: metricName, attrKey: attrKey}

	// Hash the attribute value before acquiring any lock. hashAttrValue keeps
	// Str/Int/Double/Bool/Bytes on a zero-allocation path; Map/Slice fall back
	// to AsString (JSON-marshal).
	hashVal := hashAttrValue(attrVal)

	// Route to 1/256th of the total key space.
	shard := p.getShard(metricName)

	// Phase 1: read lock — fast path for already-tracked metric+label pairs.
	shard.mu.RLock()
	t, ok := shard.trackers[key]
	shard.mu.RUnlock()

	if !ok {
		// Phase 2: write lock — only for first-time insertion into the shard.
		// Double-check after acquiring the write lock: another goroutine may
		// have inserted the same key while we waited.
		shard.mu.Lock()
		t, ok = shard.trackers[key]
		if !ok {
			if maxT := p.config.MaxTrackerCount; maxT > 0 && p.trackerCount.Load() >= int64(maxT) {
				shard.mu.Unlock()
				p.trackersRejected.Add(1)
				if p.rejectionWarned.CompareAndSwap(false, true) {
					p.logger.Warn("MaxTrackerCount reached; ignoring new metric attributes until next epoch rotation",
						zap.Int("limit", maxT))
				}
				return false
			}
			t = newTracker()
			shard.trackers[key] = t
			p.trackerCount.Add(1)
		}
		shard.mu.Unlock()
	}

	// HLL insert and estimate happen under the per-tracker lock, completely
	// independent of the shard lock.
	return t.insert(hashVal, p.getLimit(metricName))
}

// getLimit returns the per-metric cardinality limit for the given metric name,
// falling back to the global MaxCardinalityDeltaPerEpoch if no override exists.
// The map is read-only after construction — no lock needed.
func (p *cardinalityProcessor) getLimit(metricName string) uint64 {
	if v, ok := p.config.MetricOverrides[metricName]; ok {
		return uint64(v)
	}
	return uint64(p.config.MaxCardinalityDeltaPerEpoch)
}
