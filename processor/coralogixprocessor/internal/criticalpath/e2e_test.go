// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package criticalpath_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	coralogixprocessor "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/criticalpath"
)

type e2eSpanSpec struct {
	name     string
	spanID   byte
	parentID byte
	startNS  int64
	endNS    int64
}

type expectedCriticalAttrs struct {
	exclusiveNS int64
	inclusiveNS int64
}

type e2eScenario struct {
	name        string
	traceIDByte byte
	spans       []e2eSpanSpec
	expectedOn  map[string]expectedCriticalAttrs
	expectedOff []string
}

func TestCriticalPathE2E_Scenarios(t *testing.T) {
	factory := coralogixprocessor.NewFactory()
	cfg := &coralogixprocessor.Config{
		CriticalPathConfig: coralogixprocessor.CriticalPathConfig{Enabled: true},
	}

	scenarios := []e2eScenario{
		{
			name:        "jaeger_fixture_sibling_hop",
			traceIDByte: 1,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 1, endNS: 101},
				{name: "left", spanID: 2, parentID: 1, startNS: 10, endNS: 50},
				{name: "right", spanID: 3, parentID: 1, startNS: 20, endNS: 60},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 60, inclusiveNS: 100},
				"right": {exclusiveNS: 40, inclusiveNS: 40},
			},
			expectedOff: []string{"left"},
		},
		{
			name:        "vertical_chain",
			traceIDByte: 2,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 150},
				{name: "branch-a", spanID: 2, parentID: 1, startNS: 10, endNS: 150},
				{name: "branch-a-db", spanID: 3, parentID: 2, startNS: 30, endNS: 150},
				{name: "branch-a-io", spanID: 4, parentID: 3, startNS: 70, endNS: 150},
				{name: "branch-b", spanID: 5, parentID: 1, startNS: 20, endNS: 80},
				{name: "branch-c", spanID: 6, parentID: 1, startNS: 90, endNS: 120},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":        {exclusiveNS: 10, inclusiveNS: 150},
				"branch-a":    {exclusiveNS: 20, inclusiveNS: 140},
				"branch-a-db": {exclusiveNS: 40, inclusiveNS: 120},
				"branch-a-io": {exclusiveNS: 80, inclusiveNS: 80},
			},
			expectedOff: []string{"branch-b", "branch-c"},
		},
		{
			name:        "single_span",
			traceIDByte: 3,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 100},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root": {exclusiveNS: 100, inclusiveNS: 100},
			},
		},
		{
			name:        "single_child_full_tail",
			traceIDByte: 4,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 100},
				{name: "child", spanID: 2, parentID: 1, startNS: 20, endNS: 100},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 20, inclusiveNS: 100},
				"child": {exclusiveNS: 80, inclusiveNS: 80},
			},
		},
		{
			name:        "single_child_middle_gap",
			traceIDByte: 5,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 100},
				{name: "child", spanID: 2, parentID: 1, startNS: 20, endNS: 60},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 60, inclusiveNS: 100},
				"child": {exclusiveNS: 40, inclusiveNS: 40},
			},
		},
		{
			name:        "two_non_overlapping_siblings",
			traceIDByte: 6,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 1, endNS: 101},
				{name: "first", spanID: 2, parentID: 1, startNS: 20, endNS: 40},
				{name: "second", spanID: 3, parentID: 1, startNS: 50, endNS: 60},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":   {exclusiveNS: 70, inclusiveNS: 100},
				"first":  {exclusiveNS: 20, inclusiveNS: 20},
				"second": {exclusiveNS: 10, inclusiveNS: 10},
			},
		},
		{
			name:        "overlapping_siblings_latest_only",
			traceIDByte: 7,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 120},
				{name: "first", spanID: 2, parentID: 1, startNS: 20, endNS: 80},
				{name: "second", spanID: 3, parentID: 1, startNS: 50, endNS: 100},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":   {exclusiveNS: 70, inclusiveNS: 120},
				"second": {exclusiveNS: 50, inclusiveNS: 50},
			},
			expectedOff: []string{"first"},
		},
		{
			name:        "three_sibling_staircase",
			traceIDByte: 8,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 200},
				{name: "a", spanID: 2, parentID: 1, startNS: 20, endNS: 40},
				{name: "b", spanID: 3, parentID: 1, startNS: 50, endNS: 90},
				{name: "c", spanID: 4, parentID: 1, startNS: 110, endNS: 150},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root": {exclusiveNS: 100, inclusiveNS: 200},
				"a":    {exclusiveNS: 20, inclusiveNS: 20},
				"b":    {exclusiveNS: 40, inclusiveNS: 40},
				"c":    {exclusiveNS: 40, inclusiveNS: 40},
			},
		},
		{
			name:        "nested_chain_with_side_branches",
			traceIDByte: 9,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 200},
				{name: "child", spanID: 2, parentID: 1, startNS: 20, endNS: 180},
				{name: "leaf", spanID: 3, parentID: 2, startNS: 60, endNS: 160},
				{name: "side-root", spanID: 4, parentID: 1, startNS: 30, endNS: 50},
				{name: "side-child", spanID: 5, parentID: 2, startNS: 90, endNS: 110},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 40, inclusiveNS: 200},
				"child": {exclusiveNS: 60, inclusiveNS: 160},
				"leaf":  {exclusiveNS: 100, inclusiveNS: 100},
			},
			expectedOff: []string{"side-root", "side-child"},
		},
		{
			name:        "multi_root_missing_parent",
			traceIDByte: 10,
			spans: []e2eSpanSpec{
				{name: "root-a", spanID: 1, startNS: 0, endNS: 100},
				{name: "child-a", spanID: 2, parentID: 1, startNS: 20, endNS: 60},
				{name: "root-b", spanID: 3, parentID: 99, startNS: 110, endNS: 180},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root-a":  {exclusiveNS: 60, inclusiveNS: 100},
				"child-a": {exclusiveNS: 40, inclusiveNS: 40},
				"root-b":  {exclusiveNS: 70, inclusiveNS: 70},
			},
		},
		{
			name:        "truncate_child_start",
			traceIDByte: 11,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 10, endNS: 100},
				{name: "child", spanID: 2, parentID: 1, startNS: 0, endNS: 40},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 60, inclusiveNS: 90},
				"child": {exclusiveNS: 30, inclusiveNS: 30},
			},
		},
		{
			name:        "truncate_child_end",
			traceIDByte: 12,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 10, endNS: 100},
				{name: "child", spanID: 2, parentID: 1, startNS: 80, endNS: 120},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 70, inclusiveNS: 90},
				"child": {exclusiveNS: 20, inclusiveNS: 20},
			},
		},
		{
			name:        "child_covers_parent_after_truncation",
			traceIDByte: 13,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 10, endNS: 100},
				{name: "child", spanID: 2, parentID: 1, startNS: 0, endNS: 150},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 0, inclusiveNS: 90},
				"child": {exclusiveNS: 90, inclusiveNS: 90},
			},
		},
		{
			name:        "drop_child_after_parent",
			traceIDByte: 14,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 10, endNS: 100},
				{name: "drop", spanID: 2, parentID: 1, startNS: 120, endNS: 130},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root": {exclusiveNS: 90, inclusiveNS: 90},
			},
			expectedOff: []string{"drop"},
		},
		{
			name:        "deep_sibling_hop",
			traceIDByte: 15,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 200},
				{name: "a", spanID: 2, parentID: 1, startNS: 20, endNS: 120},
				{name: "b", spanID: 3, parentID: 2, startNS: 40, endNS: 80},
				{name: "c", spanID: 4, parentID: 1, startNS: 130, endNS: 180},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root": {exclusiveNS: 50, inclusiveNS: 200},
				"a":    {exclusiveNS: 60, inclusiveNS: 100},
				"b":    {exclusiveNS: 40, inclusiveNS: 40},
				"c":    {exclusiveNS: 50, inclusiveNS: 50},
			},
		},
		{
			name:        "tie_same_end_later_start_wins",
			traceIDByte: 16,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 100},
				{name: "left", spanID: 2, parentID: 1, startNS: 10, endNS: 60},
				{name: "right", spanID: 3, parentID: 1, startNS: 20, endNS: 60},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 60, inclusiveNS: 100},
				"right": {exclusiveNS: 40, inclusiveNS: 40},
			},
			expectedOff: []string{"left"},
		},
		{
			name:        "tie_same_end_same_start_higher_span_id_wins",
			traceIDByte: 17,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 100},
				{name: "left", spanID: 2, parentID: 1, startNS: 20, endNS: 60},
				{name: "right", spanID: 3, parentID: 1, startNS: 20, endNS: 60},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 60, inclusiveNS: 100},
				"right": {exclusiveNS: 40, inclusiveNS: 40},
			},
			expectedOff: []string{"left"},
		},
		{
			name:        "zero_and_invalid_children",
			traceIDByte: 18,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 100},
				{name: "child", spanID: 2, parentID: 1, startNS: 20, endNS: 50},
				{name: "zero", spanID: 3, parentID: 1, startNS: 80, endNS: 80},
				{name: "invalid", spanID: 4, parentID: 1, startNS: 90, endNS: 70},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":  {exclusiveNS: 70, inclusiveNS: 100},
				"child": {exclusiveNS: 30, inclusiveNS: 30},
			},
			expectedOff: []string{"zero", "invalid"},
		},
		{
			name:        "transformed_article_tree",
			traceIDByte: 19,
			spans: []e2eSpanSpec{
				{name: "1", spanID: 1, startNS: 0, endNS: 200},
				{name: "2", spanID: 2, parentID: 1, startNS: 10, endNS: 200},
				{name: "3", spanID: 3, parentID: 2, startNS: 20, endNS: 200},
				{name: "4", spanID: 4, parentID: 3, startNS: 30, endNS: 70},
				{name: "5", spanID: 5, parentID: 3, startNS: 30, endNS: 200},
				{name: "6", spanID: 6, parentID: 2, startNS: 25, endNS: 120},
				{name: "7", spanID: 7, parentID: 5, startNS: 40, endNS: 200},
				{name: "8", spanID: 8, parentID: 5, startNS: 45, endNS: 90},
				{name: "9", spanID: 9, parentID: 4, startNS: 40, endNS: 65},
				{name: "10", spanID: 10, parentID: 8, startNS: 55, endNS: 85},
				{name: "11", spanID: 11, parentID: 7, startNS: 50, endNS: 200},
				{name: "12", spanID: 12, parentID: 9, startNS: 50, endNS: 60},
				{name: "13", spanID: 13, parentID: 11, startNS: 60, endNS: 200},
				{name: "14", spanID: 14, parentID: 6, startNS: 35, endNS: 110},
				{name: "15", spanID: 15, parentID: 6, startNS: 45, endNS: 100},
				{name: "16", spanID: 16, parentID: 13, startNS: 70, endNS: 200},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"1":  {exclusiveNS: 10, inclusiveNS: 200},
				"2":  {exclusiveNS: 10, inclusiveNS: 190},
				"3":  {exclusiveNS: 10, inclusiveNS: 180},
				"5":  {exclusiveNS: 10, inclusiveNS: 170},
				"7":  {exclusiveNS: 10, inclusiveNS: 160},
				"11": {exclusiveNS: 10, inclusiveNS: 150},
				"13": {exclusiveNS: 10, inclusiveNS: 140},
				"16": {exclusiveNS: 130, inclusiveNS: 130},
			},
			expectedOff: []string{"4", "6", "8", "9", "10", "12", "14", "15"},
		},
		{
			name:        "complex_dual_subtrees",
			traceIDByte: 20,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 300},
				{name: "left", spanID: 2, parentID: 1, startNS: 20, endNS: 140},
				{name: "left-a", spanID: 3, parentID: 2, startNS: 40, endNS: 70},
				{name: "left-b", spanID: 4, parentID: 2, startNS: 90, endNS: 130},
				{name: "right", spanID: 5, parentID: 1, startNS: 160, endNS: 280},
				{name: "right-a", spanID: 6, parentID: 5, startNS: 180, endNS: 220},
				{name: "right-b", spanID: 7, parentID: 5, startNS: 230, endNS: 260},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root":    {exclusiveNS: 60, inclusiveNS: 300},
				"left":    {exclusiveNS: 50, inclusiveNS: 120},
				"left-a":  {exclusiveNS: 30, inclusiveNS: 30},
				"left-b":  {exclusiveNS: 40, inclusiveNS: 40},
				"right":   {exclusiveNS: 50, inclusiveNS: 120},
				"right-a": {exclusiveNS: 40, inclusiveNS: 40},
				"right-b": {exclusiveNS: 30, inclusiveNS: 30},
			},
		},
		{
			name:        "three_level_sibling_hops",
			traceIDByte: 21,
			spans: []e2eSpanSpec{
				{name: "root", spanID: 1, startNS: 0, endNS: 250},
				{name: "a", spanID: 2, parentID: 1, startNS: 10, endNS: 100},
				{name: "a1", spanID: 3, parentID: 2, startNS: 20, endNS: 40},
				{name: "a2", spanID: 4, parentID: 2, startNS: 50, endNS: 90},
				{name: "b", spanID: 5, parentID: 1, startNS: 120, endNS: 220},
				{name: "b1", spanID: 6, parentID: 5, startNS: 130, endNS: 170},
				{name: "b2", spanID: 7, parentID: 5, startNS: 180, endNS: 210},
			},
			expectedOn: map[string]expectedCriticalAttrs{
				"root": {exclusiveNS: 60, inclusiveNS: 250},
				"a":    {exclusiveNS: 30, inclusiveNS: 90},
				"a1":   {exclusiveNS: 20, inclusiveNS: 20},
				"a2":   {exclusiveNS: 40, inclusiveNS: 40},
				"b":    {exclusiveNS: 30, inclusiveNS: 100},
				"b1":   {exclusiveNS: 40, inclusiveNS: 40},
				"b2":   {exclusiveNS: 30, inclusiveNS: 30},
			},
		},
	}

	require.GreaterOrEqual(t, len(scenarios), 20)

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			sink := &consumertest.TracesSink{}
			componentInstance, err := factory.CreateTraces(
				t.Context(),
				processortest.NewNopSettings(component.MustNewType("coralogix")),
				cfg,
				sink,
			)
			require.NoError(t, err)

			traces := newTraceFromSpecs(scenario.traceIDByte, scenario.spans)
			err = componentInstance.ConsumeTraces(t.Context(), traces)
			require.NoError(t, err)

			require.Len(t, sink.AllTraces(), 1)
			got := sink.AllTraces()[0]

			for spanName, expected := range scenario.expectedOn {
				assertCriticalPathAttrs(t, e2eSpanByName(got, spanName), expected.exclusiveNS, expected.inclusiveNS)
			}

			for _, spanName := range scenario.expectedOff {
				assertNoCriticalPathAttrsE2E(t, e2eSpanByName(got, spanName))
			}
		})
	}
}

func TestCriticalPathE2E_VeryDeepChain(t *testing.T) {
	factory := coralogixprocessor.NewFactory()
	cfg := &coralogixprocessor.Config{
		CriticalPathConfig: coralogixprocessor.CriticalPathConfig{Enabled: true},
	}
	sink := &consumertest.TracesSink{}
	componentInstance, err := factory.CreateTraces(
		t.Context(),
		processortest.NewNopSettings(component.MustNewType("coralogix")),
		cfg,
		sink,
	)
	require.NoError(t, err)

	const depth = 4096
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99})
	for i := range depth {
		spec := e2eSpanSpec{
			name:     "span-" + strconv.Itoa(i),
			spanID:   byte((i % 250) + 1),
			startNS:  int64(i),
			endNS:    depth,
			parentID: 0,
		}
		if i > 0 {
			spec.parentID = byte(((i - 1) % 250) + 1)
		}
		appendE2ESpanWithIDs(spans, traceID, spec, uint64(i+1), uint64(i))
	}

	err = componentInstance.ConsumeTraces(t.Context(), traces)
	require.NoError(t, err)
	require.Len(t, sink.AllTraces(), 1)
	got := sink.AllTraces()[0]

	assertCriticalPathAttrs(t, e2eSpanByName(got, "span-0"), 1, int64(depth))
	assertCriticalPathAttrs(t, e2eSpanByName(got, "span-2048"), 1, int64(depth-2048))
	assertCriticalPathAttrs(t, e2eSpanByName(got, "span-4095"), 1, 1)
}

func newTraceFromSpecs(traceIDByte byte, specs []e2eSpanSpec) ptrace.Traces {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte, traceIDByte})

	for _, spec := range specs {
		appendE2ESpan(spans, traceID, spec)
	}

	return traces
}

func appendE2ESpan(spans ptrace.SpanSlice, traceID pcommon.TraceID, spec e2eSpanSpec) {
	span := spans.AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.SpanID([8]byte{spec.spanID}))
	if spec.parentID != 0 {
		span.SetParentSpanID(pcommon.SpanID([8]byte{spec.parentID}))
	}
	span.SetName(spec.name)
	span.SetStartTimestamp(pcommon.Timestamp(spec.startNS))
	span.SetEndTimestamp(pcommon.Timestamp(spec.endNS))
}

func appendE2ESpanWithIDs(spans ptrace.SpanSlice, traceID pcommon.TraceID, spec e2eSpanSpec, spanID, parentID uint64) {
	span := spans.AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanIDToPCommon(spanID))
	if parentID != 0 {
		span.SetParentSpanID(spanIDToPCommon(parentID))
	}
	span.SetName(spec.name)
	span.SetStartTimestamp(pcommon.Timestamp(spec.startNS))
	span.SetEndTimestamp(pcommon.Timestamp(spec.endNS))
}

func spanIDToPCommon(id uint64) pcommon.SpanID {
	return pcommon.SpanID([8]byte{
		byte(id >> 56),
		byte(id >> 48),
		byte(id >> 40),
		byte(id >> 32),
		byte(id >> 24),
		byte(id >> 16),
		byte(id >> 8),
		byte(id),
	})
}

func e2eSpanByName(traces ptrace.Traces, name string) ptrace.Span {
	resourceSpans := traces.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		scopeSpans := resourceSpans.At(i).ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.Name() == name {
					return span
				}
			}
		}
	}

	panic("span not found: " + name)
}

func assertCriticalPathAttrs(t *testing.T, span ptrace.Span, exclusiveNS, inclusiveNS int64) {
	t.Helper()

	critical, ok := span.Attributes().Get(criticalpath.AttributeCriticalPathIsOnPath)
	require.True(t, ok)
	assert.True(t, critical.Bool())

	exclusive, ok := span.Attributes().Get(criticalpath.AttributeCriticalPathExclusiveDurationNS)
	require.True(t, ok)
	assert.Equal(t, exclusiveNS, exclusive.Int())

	inclusive, ok := span.Attributes().Get(criticalpath.AttributeCriticalPathInclusiveDurationNS)
	require.True(t, ok)
	assert.Equal(t, inclusiveNS, inclusive.Int())
}

func assertNoCriticalPathAttrsE2E(t *testing.T, span ptrace.Span) {
	t.Helper()

	_, ok := span.Attributes().Get(criticalpath.AttributeCriticalPathIsOnPath)
	assert.False(t, ok)
	_, ok = span.Attributes().Get(criticalpath.AttributeCriticalPathExclusiveDurationNS)
	assert.False(t, ok)
	_, ok = span.Attributes().Get(criticalpath.AttributeCriticalPathInclusiveDurationNS)
	assert.False(t, ok)
}
