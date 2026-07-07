// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// DEMO ONLY, keep untracked, do not commit. Run with:
//
//	go test -run TestPrintDemo -v
package spanpruningprocessor

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

func TestPrintDemo(t *testing.T) {
	fmt.Println("\n================ INPUT ================")
	printTrace(buildDemoTrace())

	fmt.Println("\n================ OUTPUT: outliers OFF ================")
	runDemo(t, func(c *Config) { c.EnableOutlierAnalysis = false })

	fmt.Println("\n================ OUTPUT: outliers ON + preserve ================")
	runDemo(t, func(c *Config) {
		c.EnableOutlierAnalysis = true
		c.OutlierAnalysis = OutlierAnalysisConfig{
			Method:                         OutlierMethodIQR,
			IQRMultiplier:                  1.5,
			MinGroupSize:                   5,
			PreserveOutliers:               true,
			MaxPreservedOutliers:           0, // preserve all detected
			CorrelationMinOccurrence:       0.5,
			CorrelationMaxNormalOccurrence: 0.5,
			MaxCorrelatedAttributes:        5,
			MinOutlierThresholdPercent:     0.1,
		}
	})
}

func runDemo(t *testing.T, configure func(*Config)) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 4
	cfg.MaxParentDepth = -1
	configure(cfg)

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	td := buildDemoTrace()
	if err := tp.ConsumeTraces(context.Background(), td); err != nil { //nolint:usetesting
		t.Fatal(err)
	}
	printTrace(td)
}

// buildDemoTrace builds:
//
//	root
//	├── handler (50ms) x6 normal           each with 4 normal query leaves (5ms)
//	├── handler (50ms) "slow-query"        queries 5,5,5,45ms  -> 45ms is a LEAF outlier
//	└── handler (600ms) "slow-handler"     queries 5,5,5,550ms -> INTERIOR outlier + nested 550ms leaf
func buildDemoTrace() ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	var idCounter uint16
	nextID := func() pcommon.SpanID {
		idCounter++
		var id [8]byte
		id[0] = byte(idCounter)
		id[1] = byte(idCounter >> 8)
		return pcommon.SpanID(id)
	}

	base := int64(1_000_000_000)
	ms := int64(1_000_000)
	add := func(id, parent pcommon.SpanID, name, label string, durMs int64) {
		s := ss.Spans().AppendEmpty()
		s.SetTraceID(traceID)
		s.SetSpanID(id)
		s.SetParentSpanID(parent)
		s.SetName(name)
		s.SetStartTimestamp(pcommon.Timestamp(base))
		s.SetEndTimestamp(pcommon.Timestamp(base + durMs*ms))
		if label != "" {
			s.Attributes().PutStr("label", label)
		}
	}

	root := nextID()
	add(root, pcommon.SpanID{}, "root", "", 1000)

	addHandler := func(handlerDur int64, label string, queryDurs []int64) {
		h := nextID()
		add(h, root, "handler", label, handlerDur)
		for _, qd := range queryDurs {
			ql := ""
			if qd >= 40 {
				ql = "slow-query"
			}
			add(nextID(), h, "query", ql, qd)
		}
	}

	for range 6 {
		addHandler(50, "", []int64{5, 5, 5, 5})
	}
	addHandler(50, "handler-with-slow-query", []int64{5, 5, 5, 45})
	addHandler(600, "slow-handler", []int64{5, 5, 5, 550})

	return td
}

// printTrace renders td as an indented tree with key aggregation annotations.
func printTrace(td ptrace.Traces) {
	byID := map[pcommon.SpanID]ptrace.Span{}
	children := map[pcommon.SpanID][]pcommon.SpanID{}
	var roots []pcommon.SpanID
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		for j := 0; j < rss.At(i).ScopeSpans().Len(); j++ {
			spans := rss.At(i).ScopeSpans().At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				s := spans.At(k)
				byID[s.SpanID()] = s
				if p := s.ParentSpanID(); p.IsEmpty() {
					roots = append(roots, s.SpanID())
				} else {
					children[p] = append(children[p], s.SpanID())
				}
			}
		}
	}
	// Spans whose parent was pruned become display roots.
	for id, s := range byID {
		if p := s.ParentSpanID(); !p.IsEmpty() {
			if _, ok := byID[p]; !ok {
				roots = append(roots, id)
			}
		}
	}
	sortIDs := func(ids []pcommon.SpanID) {
		sort.Slice(ids, func(a, b int) bool {
			x, y := ids[a], ids[b]
			if xs, ys := byID[x].StartTimestamp(), byID[y].StartTimestamp(); xs != ys {
				return xs < ys
			}
			return x.String() < y.String()
		})
	}
	sortIDs(roots)

	var walk func(id pcommon.SpanID, prefix string)
	walk = func(id pcommon.SpanID, prefix string) {
		s := byID[id]
		durMs := (int64(s.EndTimestamp()) - int64(s.StartTimestamp())) / 1_000_000
		tags := ""
		if v, ok := s.Attributes().Get("aggregation.is_summary"); ok && v.Bool() {
			cnt, _ := s.Attributes().Get("aggregation.span_count")
			tags += fmt.Sprintf(" [SUMMARY n=%d]", cnt.Int())
			if poc, ok := s.Attributes().Get("aggregation.preserved_outlier_count"); ok {
				tags += fmt.Sprintf(" preserved_outlier_count=%d", poc.Int())
			}
		}
		if v, ok := s.Attributes().Get("aggregation.is_preserved_outlier"); ok && v.Bool() {
			tags += " [PRESERVED_OUTLIER]"
		}
		if l, ok := s.Attributes().Get("label"); ok {
			tags += " label=" + l.AsString()
		}
		fmt.Printf("%s%s (%dms)%s\n", prefix, s.Name(), durMs, tags)
		kids := children[id]
		sortIDs(kids)
		for _, kid := range kids {
			walk(kid, prefix+"  ")
		}
	}
	for _, r := range roots {
		walk(r, "")
	}
}
