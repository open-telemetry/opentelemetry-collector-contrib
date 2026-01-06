// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// spanInfo holds a span and its location within the trace data structure
type spanInfo struct {
	span          ptrace.Span
	scopeSpans    ptrace.ScopeSpans
	resourceSpans ptrace.ResourceSpans
}

// spanGroup holds a group of similar leaf spans for aggregation
type spanGroup struct {
	spans    []spanInfo
	groupKey string
}

// aggregationStats holds statistics for a group of spans
type aggregationStats struct {
	count       int64
	minDuration time.Duration
	maxDuration time.Duration
	sumDuration time.Duration
}

// attributePattern holds a compiled glob pattern for matching attribute keys
type attributePattern struct {
	pattern string
	glob    glob.Glob
}

type leafSpanPruningProcessor struct {
	config            *Config
	logger            *zap.Logger
	attributePatterns []attributePattern
}

func newLeafSpanPruningProcessor(set processor.Settings, cfg *Config) (*leafSpanPruningProcessor, error) {
	// Compile glob patterns for group_by_attributes
	patterns := make([]attributePattern, 0, len(cfg.GroupByAttributes))
	for _, pattern := range cfg.GroupByAttributes {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		patterns = append(patterns, attributePattern{
			pattern: pattern,
			glob:    g,
		})
	}

	return &leafSpanPruningProcessor{
		config:            cfg,
		logger:            set.Logger,
		attributePatterns: patterns,
	}, nil
}

func (p *leafSpanPruningProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	// Group spans by TraceID
	traceSpans := p.groupSpansByTraceID(td)

	// Process each trace independently
	for _, spans := range traceSpans {
		if err := p.processTrace(ctx, spans, td); err != nil {
			return td, err
		}
	}

	return td, nil
}

// groupSpansByTraceID collects all spans organized by trace ID
func (p *leafSpanPruningProcessor) groupSpansByTraceID(td ptrace.Traces) map[pcommon.TraceID][]spanInfo {
	traceSpans := make(map[pcommon.TraceID][]spanInfo)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				traceID := span.TraceID()
				traceSpans[traceID] = append(traceSpans[traceID], spanInfo{
					span:          span,
					scopeSpans:    ils,
					resourceSpans: rs,
				})
			}
		}
	}

	return traceSpans
}

// processTrace processes a single trace, identifying and aggregating similar leaf spans
func (p *leafSpanPruningProcessor) processTrace(ctx context.Context, spans []spanInfo, td ptrace.Traces) error {
	// Step 1: Identify leaf spans (spans not referenced as parent by any other span)
	leafSpans := p.findLeafSpans(spans)

	if len(leafSpans) == 0 {
		return nil
	}

	// Step 2: Group similar leaf spans by name + configured attributes
	groups := p.groupSimilarSpans(leafSpans)

	// Step 3: For each group meeting minimum threshold, create summary span and remove originals
	for _, group := range groups {
		if len(group.spans) >= p.config.MinSpansToAggregate {
			p.aggregateGroup(group)
		}
	}

	return nil
}

// findLeafSpans identifies spans that have no children
func (p *leafSpanPruningProcessor) findLeafSpans(spans []spanInfo) []spanInfo {
	// Build set of all span IDs that are referenced as parents
	parentSpanIDs := make(map[pcommon.SpanID]struct{})
	for _, info := range spans {
		parentID := info.span.ParentSpanID()
		if !parentID.IsEmpty() {
			parentSpanIDs[parentID] = struct{}{}
		}
	}

	// Leaf spans are those whose SpanID is NOT in parentSpanIDs
	var leafSpans []spanInfo
	for _, info := range spans {
		if _, isParent := parentSpanIDs[info.span.SpanID()]; !isParent {
			leafSpans = append(leafSpans, info)
		}
	}

	return leafSpans
}

// groupSimilarSpans groups spans by name and configured attributes
func (p *leafSpanPruningProcessor) groupSimilarSpans(spans []spanInfo) []spanGroup {
	groups := make(map[string]*spanGroup)

	for _, info := range spans {
		key := p.buildGroupKey(info.span)
		if group, exists := groups[key]; exists {
			group.spans = append(group.spans, info)
		} else {
			groups[key] = &spanGroup{
				spans:    []spanInfo{info},
				groupKey: key,
			}
		}
	}

	// Convert map to slice
	result := make([]spanGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}
	return result
}

// buildGroupKey creates a grouping key from span name and matching attributes
// Attributes are matched using glob patterns from the configuration
func (p *leafSpanPruningProcessor) buildGroupKey(span ptrace.Span) string {
	var builder strings.Builder
	builder.WriteString(span.Name())

	attrs := span.Attributes()

	// Collect all matching attribute key-value pairs
	matchedAttrs := make(map[string]string)
	attrs.Range(func(key string, value pcommon.Value) bool {
		for _, pattern := range p.attributePatterns {
			if pattern.glob.Match(key) {
				matchedAttrs[key] = value.AsString()
				break // Only match each key once
			}
		}
		return true
	})

	// Sort keys for consistent ordering in the group key
	keys := make([]string, 0, len(matchedAttrs))
	for k := range matchedAttrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build the group key with sorted attribute key-value pairs
	for _, key := range keys {
		builder.WriteString("|")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(matchedAttrs[key])
	}

	return builder.String()
}

// aggregateGroup creates a summary span and removes the original spans
func (p *leafSpanPruningProcessor) aggregateGroup(group spanGroup) {
	// Calculate statistics
	stats := p.calculateStats(group.spans)

	// Create summary span using the first span's location
	p.createSummarySpan(group, stats)

	// Mark original spans for removal by setting a flag
	// We remove by setting an empty name which we'll filter out
	for _, info := range group.spans {
		// Use RemoveIf pattern: mark spans to remove
		info.span.SetName("")
	}

	// Remove marked spans from their scope spans
	for _, info := range group.spans {
		info.scopeSpans.Spans().RemoveIf(func(span ptrace.Span) bool {
			return span.Name() == ""
		})
	}
}

// calculateStats computes aggregation statistics for a group of spans
func (p *leafSpanPruningProcessor) calculateStats(spans []spanInfo) aggregationStats {
	stats := aggregationStats{count: int64(len(spans))}

	for i, info := range spans {
		startTime := info.span.StartTimestamp().AsTime()
		endTime := info.span.EndTimestamp().AsTime()
		duration := endTime.Sub(startTime)

		if i == 0 {
			stats.minDuration = duration
			stats.maxDuration = duration
		} else {
			if duration < stats.minDuration {
				stats.minDuration = duration
			}
			if duration > stats.maxDuration {
				stats.maxDuration = duration
			}
		}
		stats.sumDuration += duration
	}

	return stats
}

// createSummarySpan creates a new summary span with aggregation statistics
func (p *leafSpanPruningProcessor) createSummarySpan(group spanGroup, stats aggregationStats) {
	// Use the first span as a template
	templateSpan := group.spans[0].span
	scopeSpans := group.spans[0].scopeSpans

	// Create new span in the same ScopeSpans as the first span
	newSpan := scopeSpans.Spans().AppendEmpty()

	// Copy basic properties from template
	newSpan.SetName(templateSpan.Name() + p.config.SummarySpanNameSuffix)
	newSpan.SetTraceID(templateSpan.TraceID())
	newSpan.SetSpanID(generateSpanID())
	newSpan.SetParentSpanID(templateSpan.ParentSpanID())
	newSpan.SetKind(templateSpan.Kind())

	// Set timestamps: earliest start, latest end
	earliestStart, latestEnd := p.findTimeRange(group.spans)
	newSpan.SetStartTimestamp(earliestStart)
	newSpan.SetEndTimestamp(latestEnd)

	// Copy attributes from template
	templateSpan.Attributes().CopyTo(newSpan.Attributes())

	// Add aggregation statistics as attributes
	prefix := p.config.AggregationAttributePrefix
	newSpan.Attributes().PutInt(prefix+"span_count", stats.count)
	newSpan.Attributes().PutInt(prefix+"duration_min_ns", int64(stats.minDuration))
	newSpan.Attributes().PutInt(prefix+"duration_max_ns", int64(stats.maxDuration))
	newSpan.Attributes().PutInt(prefix+"duration_total_ns", int64(stats.sumDuration))
	if stats.count > 0 {
		newSpan.Attributes().PutInt(prefix+"duration_avg_ns", int64(stats.sumDuration)/stats.count)
	}

	// Set status: if any span had error, summary has error
	p.aggregateStatus(group.spans, newSpan)
}

// findTimeRange finds the earliest start and latest end timestamps in a group
func (p *leafSpanPruningProcessor) findTimeRange(spans []spanInfo) (pcommon.Timestamp, pcommon.Timestamp) {
	var earliestStart, latestEnd pcommon.Timestamp

	for i, info := range spans {
		start := info.span.StartTimestamp()
		end := info.span.EndTimestamp()

		if i == 0 {
			earliestStart = start
			latestEnd = end
		} else {
			if start < earliestStart {
				earliestStart = start
			}
			if end > latestEnd {
				latestEnd = end
			}
		}
	}

	return earliestStart, latestEnd
}

// aggregateStatus sets the summary span status based on original spans
// If any span had an error, the summary has an error
func (p *leafSpanPruningProcessor) aggregateStatus(spans []spanInfo, summarySpan ptrace.Span) {
	hasError := false
	hasOk := false
	var errorMessage string

	for _, info := range spans {
		status := info.span.Status()
		switch status.Code() {
		case ptrace.StatusCodeError:
			hasError = true
			if errorMessage == "" {
				errorMessage = status.Message()
			}
		case ptrace.StatusCodeOk:
			hasOk = true
		}
	}

	if hasError {
		summarySpan.Status().SetCode(ptrace.StatusCodeError)
		summarySpan.Status().SetMessage(errorMessage)
	} else if hasOk {
		summarySpan.Status().SetCode(ptrace.StatusCodeOk)
	} else {
		summarySpan.Status().SetCode(ptrace.StatusCodeUnset)
	}
}

// generateSpanID creates a new random span ID
func generateSpanID() pcommon.SpanID {
	var id [8]byte
	_, _ = rand.Read(id[:])
	return pcommon.SpanID(id)
}