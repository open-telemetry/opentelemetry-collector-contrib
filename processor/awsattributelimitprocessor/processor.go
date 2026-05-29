// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsattributelimitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsattributelimitprocessor"

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Attribute source constants used for classification and switch dispatch.
const (
	attrSourceResource  = "resource"
	attrSourceScope     = "scope"
	attrSourceDatapoint = "datapoint"
)

// attributeLimitProcessor enforces the aws backend attribute limit by removing
// redundant attributes and dropping low-priority attributes
// by tier when the total count exceeds the configured maximum.
type attributeLimitProcessor struct {
	config                       *Config
	logger                       *zap.Logger
	unconditionalRemovalKeys     map[string]struct{}
	unconditionalRemovalPrefixes []string
	mu                           sync.Mutex
	lastLogAt                    map[string]time.Time // rate-limiting: last log time per metric name
	lastEviction                 time.Time
}

func newProcessor(cfg *Config, logger *zap.Logger) *attributeLimitProcessor {
	keySet := make(map[string]struct{}, len(cfg.UnconditionalRemovalKeys))
	for _, k := range cfg.UnconditionalRemovalKeys {
		keySet[k] = struct{}{}
	}
	return &attributeLimitProcessor{
		config:                       cfg,
		logger:                       logger,
		unconditionalRemovalKeys:     keySet,
		unconditionalRemovalPrefixes: cfg.UnconditionalRemovalPrefixes,
		lastLogAt:                    make(map[string]time.Time),
		lastEviction:                 time.Now(),
	}
}

// Start is a no-op for this processor (no external resources to initialize).
func (p *attributeLimitProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown is a no-op for this processor (no external resources to release).
func (p *attributeLimitProcessor) Shutdown(_ context.Context) error {
	return nil
}

// removeUnconditionalAttributes removes all resource attributes that match
// unconditional removal rules (prefix patterns and exact keys) in a single pass.
func (p *attributeLimitProcessor) removeUnconditionalAttributes(attrs pcommon.Map) {
	attrs.RemoveIf(func(key string, _ pcommon.Value) bool {
		// Check exact key match first (O(1) map lookup).
		if _, ok := p.unconditionalRemovalKeys[key]; ok {
			return true
		}
		// Check prefix patterns.
		for _, prefix := range p.unconditionalRemovalPrefixes {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
		return false
	})
}

// attrEntry represents a droppable attribute with its tier and source.
type attrEntry struct {
	key    string
	tier   int
	source string // "datapoint", "scope", or "resource"
}

// attrEntryPool reduces allocations when Phase 2 runs frequently.
// We store *[]attrEntry (pointer to slice) rather than []attrEntry because
// sync.Pool stores interface{} values — storing the slice directly would cause
// the slice header to be boxed into an interface on every Get/Put, defeating
// the purpose. Storing a pointer avoids this allocation.
var attrEntryPool = sync.Pool{
	New: func() any {
		s := make([]attrEntry, 0, 64)
		return &s
	},
}

// removeExcessByTier collects droppable attributes from datapoint, scope, and resource,
// sorts by tier then alphabetically, and removes them until the excess count is satisfied.
// Returns the number of attributes dropped and the min/max tier used.
func removeExcessByTier(resourceAttrs, scopeAttrs, datapointAttrs pcommon.Map, excess int) (droppedCount, minTier, maxTier int) {
	droppablePtr := attrEntryPool.Get().(*[]attrEntry)
	droppable := (*droppablePtr)[:0]
	defer func() {
		*droppablePtr = droppable[:0]
		attrEntryPool.Put(droppablePtr)
	}()

	// Scan resource attributes (tiers 1-8).
	resourceAttrs.Range(func(key string, _ pcommon.Value) bool {
		tier := classifyAttribute(key, attrSourceResource)
		if tier > 0 {
			droppable = append(droppable, attrEntry{key: key, tier: tier, source: attrSourceResource})
		}
		return true
	})

	// Scan scope attributes (tier 9).
	scopeAttrs.Range(func(key string, _ pcommon.Value) bool {
		tier := classifyAttribute(key, attrSourceScope)
		if tier > 0 {
			droppable = append(droppable, attrEntry{key: key, tier: tier, source: attrSourceScope})
		}
		return true
	})

	// Scan datapoint attributes (tier 10).
	datapointAttrs.Range(func(key string, _ pcommon.Value) bool {
		tier := classifyAttribute(key, attrSourceDatapoint)
		if tier > 0 {
			droppable = append(droppable, attrEntry{key: key, tier: tier, source: attrSourceDatapoint})
		}
		return true
	})

	// Sort by tier ascending, then alphabetically within tier.
	slices.SortFunc(droppable, func(a, b attrEntry) int {
		if a.tier != b.tier {
			return a.tier - b.tier
		}
		return strings.Compare(a.key, b.key)
	})

	// Drop until excess is satisfied.
	dropped := 0
	minT, maxT := 0, 0
	for _, entry := range droppable {
		if dropped >= excess {
			break
		}
		switch entry.source {
		case attrSourceDatapoint:
			datapointAttrs.Remove(entry.key)
		case attrSourceScope:
			scopeAttrs.Remove(entry.key)
		case attrSourceResource:
			resourceAttrs.Remove(entry.key)
		}
		dropped++
		if minT == 0 {
			minT = entry.tier
		}
		maxT = entry.tier
	}

	return dropped, minT, maxT
}

// logDropWarning emits a rate-limited warning when tier-based dropping occurs.
// Logs at most once per metric name per minute.
func (p *attributeLimitProcessor) logDropWarning(metricName string, droppedCount, minTier, maxTier int) {
	if !p.shouldLog(metricName) {
		return
	}
	p.logger.Warn("dropped attributes to meet limit",
		zap.String("metric", metricName),
		zap.Int("dropped", droppedCount),
		zap.Int("minTier", minTier),
		zap.Int("maxTier", maxTier),
		zap.Int("limit", p.config.MaxTotalAttributes),
	)
}

// logExhaustedError logs a rate-limited error when all tiers are exhausted
// and force-pruning was required to meet the limit.
func (p *attributeLimitProcessor) logExhaustedError(metricName string, remaining, forcePruned int) {
	errorKey := metricName + ":exhausted"
	if !p.shouldLog(errorKey) {
		return
	}
	p.logger.Error("all tiers exhausted, force-pruned protected attributes to meet limit",
		zap.String("metric", metricName),
		zap.Int("attributesBeforePrune", remaining),
		zap.Int("forcePruned", forcePruned),
		zap.Int("limit", p.config.MaxTotalAttributes),
	)
}

// shouldLog checks rate limiting and evicts stale entries. Returns true if
// the caller should proceed with logging. Releases the mutex before returning.
func (p *attributeLimitProcessor) shouldLog(key string) bool {
	p.mu.Lock()
	now := time.Now()
	if now.Sub(p.lastEviction) > 5*time.Minute {
		for name, lastTime := range p.lastLogAt {
			if now.Sub(lastTime) > 5*time.Minute {
				delete(p.lastLogAt, name)
			}
		}
		p.lastEviction = now
	}
	if lastTime, ok := p.lastLogAt[key]; ok && now.Sub(lastTime) < time.Minute {
		p.mu.Unlock()
		return false
	}
	p.lastLogAt[key] = now
	p.mu.Unlock()
	return true
}

// enforceLimit checks if the total attribute count exceeds the limit and runs
// Phase 2 tier-based dropping if needed.
func (p *attributeLimitProcessor) enforceLimit(resourceAttrs, scopeAttrs, datapointAttrs pcommon.Map, metricName string) {
	total := resourceAttrs.Len() + scopeAttrs.Len() + datapointAttrs.Len()
	if total <= p.config.MaxTotalAttributes {
		return
	}

	excess := total - p.config.MaxTotalAttributes
	droppedCount, minTier, maxTier := removeExcessByTier(resourceAttrs, scopeAttrs, datapointAttrs, excess)

	if droppedCount > 0 {
		p.logDropWarning(metricName, droppedCount, minTier, maxTier)
	}

	// If still over limit after tier-based dropping, force-prune remaining
	// attributes (including protected ones) to guarantee we never exceed the limit.
	remaining := resourceAttrs.Len() + scopeAttrs.Len() + datapointAttrs.Len()
	if remaining > p.config.MaxTotalAttributes {
		forcePruned := p.forcePrune(resourceAttrs, scopeAttrs, datapointAttrs)
		p.logExhaustedError(metricName, remaining, forcePruned)
	}
}

// forcePrune is a last-resort safety net that should never activate in practice.
// By the time forcePrune is called, all tier-based dropping has been exhausted,
// meaning every remaining attribute is protected. This function exists only to
// guarantee the 150-attribute hard limit is never exceeded — if it runs, it means
// the metric has 150+ protected attributes, which indicates a misconfiguration
// (too many attributes marked as protected) rather than normal operation.
//
// The drop order here is intentionally the opposite of tier-based dropping.
// In tier-based dropping, resource labels are dropped first because they are
// low-value metadata that customers rarely query by. But in forcePrune, every
// remaining attribute is protected and important — so we minimize blast radius
// by dropping datapoint attrs first (per-datapoint, no shared impact), then
// scope, then resource last (shared across all datapoints in the batch).
//
// Within each map, keys are sorted alphabetically and removed from the end.
// Returns the number of attributes force-pruned.
func (p *attributeLimitProcessor) forcePrune(resourceAttrs, scopeAttrs, datapointAttrs pcommon.Map) int {
	excess := resourceAttrs.Len() + scopeAttrs.Len() + datapointAttrs.Len() - p.config.MaxTotalAttributes
	if excess <= 0 {
		return 0
	}

	pruned := 0

	// Prune datapoint attributes first (per-datapoint, least shared impact).
	pruned += pruneFromMap(datapointAttrs, excess-pruned)

	// Then scope attributes.
	if pruned < excess {
		pruned += pruneFromMap(scopeAttrs, excess-pruned)
	}

	// Then resource attributes last (most shared impact).
	if pruned < excess {
		pruned += pruneFromMap(resourceAttrs, excess-pruned)
	}

	return pruned
}

// pruneFromMap removes up to `count` attributes from the map, sorted alphabetically
// (removes last keys first). Returns the number actually removed.
func pruneFromMap(attrs pcommon.Map, count int) int {
	if count <= 0 || attrs.Len() == 0 {
		return 0
	}

	keys := make([]string, 0, attrs.Len())
	attrs.Range(func(key string, _ pcommon.Value) bool {
		keys = append(keys, key)
		return true
	})
	slices.Sort(keys)

	// Remove from the end (alphabetically last).
	removed := 0
	for i := len(keys) - 1; i >= 0 && removed < count; i-- {
		attrs.Remove(keys[i])
		removed++
	}
	return removed
}

// processDatapoints is a generic helper that enforces the attribute limit on
// all datapoints in a slice. It works with any datapoint type that exposes
// Attributes() pcommon.Map.
func processDatapoints[DP interface{ Attributes() pcommon.Map }](
	datapoints interface {
		Len() int
		At(int) DP
	},
	p *attributeLimitProcessor,
	resourceAttrs pcommon.Map,
	scopeAttrs pcommon.Map,
	metricName string,
) {
	for i := 0; i < datapoints.Len(); i++ {
		dp := datapoints.At(i)
		p.enforceLimit(resourceAttrs, scopeAttrs, dp.Attributes(), metricName)
	}
}

func (p *attributeLimitProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceAttrs := rm.Resource().Attributes()

		// Unconditional removal (always runs).
		p.removeUnconditionalAttributes(resourceAttrs)

		// Phase 2: Per-datapoint evaluation.
		// Note: Resource attributes are shared across all datapoints in a ResourceMetrics.
		// If an earlier datapoint triggers Phase 2 and drops resource attributes, subsequent
		// datapoints see the already-trimmed resource attributes. This is by design — if any
		// datapoint is over the limit, the shared resource attributes need trimming regardless.
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeAttrs := sm.Scope().Attributes()

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				metricName := m.Name()

				switch m.Type() {
				case pmetric.MetricTypeGauge:
					processDatapoints(m.Gauge().DataPoints(), p, resourceAttrs, scopeAttrs, metricName)
				case pmetric.MetricTypeSum:
					processDatapoints(m.Sum().DataPoints(), p, resourceAttrs, scopeAttrs, metricName)
				case pmetric.MetricTypeHistogram:
					processDatapoints(m.Histogram().DataPoints(), p, resourceAttrs, scopeAttrs, metricName)
				case pmetric.MetricTypeExponentialHistogram:
					processDatapoints(m.ExponentialHistogram().DataPoints(), p, resourceAttrs, scopeAttrs, metricName)
				case pmetric.MetricTypeSummary:
					processDatapoints(m.Summary().DataPoints(), p, resourceAttrs, scopeAttrs, metricName)
				default:
					// Skip metrics with unsupported or empty type without error.
				}
			}
		}
	}
	return md, nil
}
