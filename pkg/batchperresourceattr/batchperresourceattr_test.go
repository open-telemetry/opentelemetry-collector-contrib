// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batchperresourceattr

import (
	"context"
	"errors"
	"math/rand/v2"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSplitTracesOneResourceSpans(t *testing.T) {
	inBatch := ptrace.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1")

	sink := new(consumertest.TracesSink)
	bpr := NewBatchPerResourceTraces("attr_key", sink)
	assert.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))
	outBatches := sink.AllTraces()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestOriginalResourceSpansUnchanged(t *testing.T) {
	inBatch := ptrace.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1")

	sink := new(consumertest.TracesSink)
	bpr := NewBatchPerResourceTraces("attr_key", sink)
	assert.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))
	outBatches := sink.AllTraces()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitTracesReturnError(t *testing.T) {
	inBatch := ptrace.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1")

	err := errors.New("test_error")
	bpr := NewBatchPerResourceTraces("attr_key", consumertest.NewErr(err))
	assert.Equal(t, err, bpr.ConsumeTraces(t.Context(), inBatch))
}

func TestSplitTracesSameResource(t *testing.T) {
	inBatch := ptrace.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "same_attr_val", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "same_attr_val", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "same_attr_val", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "same_attr_val", "1")
	expected := ptrace.NewTraces()
	inBatch.CopyTo(expected)

	sink := new(consumertest.TracesSink)
	bpr := NewBatchPerResourceTraces("same_attr_val", sink)
	assert.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))
	outBatches := sink.AllTraces()
	require.Len(t, outBatches, 1)
	assert.Equal(t, expected, outBatches[0])
}

func TestSplitTracesIntoDifferentBatches(t *testing.T) {
	inBatch := ptrace.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "2")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "3")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "4")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "2")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "3")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "4")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "diff_attr_key", "1")
	expected := ptrace.NewTraces()
	inBatch.CopyTo(expected)

	sink := new(consumertest.TracesSink)
	bpr := NewBatchPerResourceTraces("attr_key", sink)
	assert.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))
	outBatches := sink.AllTraces()
	require.Len(t, outBatches, 5)
	sortTraces(outBatches, "attr_key")
	assert.Equal(t, newTraces(expected.ResourceSpans().At(8)), outBatches[0])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(0), expected.ResourceSpans().At(4)), outBatches[1])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(1), expected.ResourceSpans().At(5)), outBatches[2])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(2), expected.ResourceSpans().At(6)), outBatches[3])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(3), expected.ResourceSpans().At(7)), outBatches[4])
}

func TestSplitTracesIntoDifferentBatchesWithMultipleKeys(t *testing.T) {
	inBatch := ptrace.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1", "attr_key2", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "2", "attr_key2", "2")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "3", "attr_key2", "3")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "4", "attr_key2", "4")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "1", "attr_key2", "1")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "2", "attr_key2", "2")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "3", "attr_key2", "3")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "4", "attr_key2", "4")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "5", "attr_key2", "6")
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "diff_attr_key", "1")
	expected := ptrace.NewTraces()
	inBatch.CopyTo(expected)

	sink := new(consumertest.TracesSink)
	bpr := NewMultiBatchPerResourceTraces([]string{"attr_key", "attr_key2"}, sink)
	assert.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))
	outBatches := sink.AllTraces()
	require.Len(t, outBatches, 6)
	sortTraces(outBatches, "attr_key")
	assert.Equal(t, newTraces(expected.ResourceSpans().At(9)), outBatches[0])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(0), expected.ResourceSpans().At(4)), outBatches[1])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(1), expected.ResourceSpans().At(5)), outBatches[2])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(2), expected.ResourceSpans().At(6)), outBatches[3])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(3), expected.ResourceSpans().At(7)), outBatches[4])
	assert.Equal(t, newTraces(expected.ResourceSpans().At(8)), outBatches[5])
}

func TestSplitMetricsOneResourceMetrics(t *testing.T) {
	inBatch := pmetric.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1")
	expected := pmetric.NewMetrics()
	inBatch.CopyTo(expected)

	sink := new(consumertest.MetricsSink)
	bpr := NewBatchPerResourceMetrics("attr_key", sink)
	assert.NoError(t, bpr.ConsumeMetrics(t.Context(), inBatch))
	outBatches := sink.AllMetrics()
	require.Len(t, outBatches, 1)
	assert.Equal(t, expected, outBatches[0])
}

func TestOriginalResourceMetricsUnchanged(t *testing.T) {
	inBatch := pmetric.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1")

	sink := new(consumertest.MetricsSink)
	bpr := NewBatchPerResourceMetrics("attr_key", sink)
	assert.NoError(t, bpr.ConsumeMetrics(t.Context(), inBatch))
	outBatches := sink.AllMetrics()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitMetricsReturnError(t *testing.T) {
	inBatch := pmetric.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1")

	err := errors.New("test_error")
	bpr := NewBatchPerResourceMetrics("attr_key", consumertest.NewErr(err))
	assert.Equal(t, err, bpr.ConsumeMetrics(t.Context(), inBatch))
}

func TestSplitMetricsSameResource(t *testing.T) {
	inBatch := pmetric.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "same_attr_val", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "same_attr_val", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "same_attr_val", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "same_attr_val", "1")
	expected := pmetric.NewMetrics()
	inBatch.CopyTo(expected)

	sink := new(consumertest.MetricsSink)
	bpr := NewBatchPerResourceMetrics("same_attr_val", sink)
	assert.NoError(t, bpr.ConsumeMetrics(t.Context(), inBatch))
	outBatches := sink.AllMetrics()
	require.Len(t, outBatches, 1)
	assert.Equal(t, expected, outBatches[0])
}

func TestSplitMetricsIntoDifferentBatches(t *testing.T) {
	inBatch := pmetric.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "2")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "3")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "4")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "2")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "3")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "4")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "diff_attr_key", "1")
	expected := pmetric.NewMetrics()
	inBatch.CopyTo(expected)

	sink := new(consumertest.MetricsSink)
	bpr := NewBatchPerResourceMetrics("attr_key", sink)
	assert.NoError(t, bpr.ConsumeMetrics(t.Context(), inBatch))
	outBatches := sink.AllMetrics()
	require.Len(t, outBatches, 5)
	sortMetrics(outBatches, "attr_key")
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(8)), outBatches[0])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(0), expected.ResourceMetrics().At(4)), outBatches[1])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(1), expected.ResourceMetrics().At(5)), outBatches[2])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(2), expected.ResourceMetrics().At(6)), outBatches[3])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(3), expected.ResourceMetrics().At(7)), outBatches[4])
}

func TestSplitMetricsIntoDifferentBatchesWithMultipleKeys(t *testing.T) {
	inBatch := pmetric.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1", "attr_key2", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "2", "attr_key2", "2")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "3", "attr_key2", "3")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "4", "attr_key2", "4")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "1", "attr_key2", "1")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "2", "attr_key2", "2")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "3", "attr_key2", "3")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "4", "attr_key2", "4")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "5", "attr_key2", "6")
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "diff_attr_key", "1")
	expected := pmetric.NewMetrics()
	inBatch.CopyTo(expected)

	sink := new(consumertest.MetricsSink)
	bpr := NewMultiBatchPerResourceMetrics([]string{"attr_key", "attr_key2"}, sink)
	assert.NoError(t, bpr.ConsumeMetrics(t.Context(), inBatch))
	outBatches := sink.AllMetrics()
	require.Len(t, outBatches, 6)
	sortMetrics(outBatches, "attr_key")
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(9)), outBatches[0])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(0), expected.ResourceMetrics().At(4)), outBatches[1])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(1), expected.ResourceMetrics().At(5)), outBatches[2])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(2), expected.ResourceMetrics().At(6)), outBatches[3])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(3), expected.ResourceMetrics().At(7)), outBatches[4])
	assert.Equal(t, newMetrics(expected.ResourceMetrics().At(8)), outBatches[5])
}

func TestSplitLogsOneResourceLogs(t *testing.T) {
	inBatch := plog.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1")
	expected := plog.NewLogs()
	inBatch.CopyTo(expected)

	sink := new(consumertest.LogsSink)
	bpr := NewBatchPerResourceLogs("attr_key", sink)
	assert.NoError(t, bpr.ConsumeLogs(t.Context(), inBatch))
	outBatches := sink.AllLogs()
	require.Len(t, outBatches, 1)
	assert.Equal(t, expected, outBatches[0])
}

func TestOriginalResourceLogsUnchanged(t *testing.T) {
	inBatch := plog.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1")

	sink := new(consumertest.LogsSink)
	bpr := NewBatchPerResourceLogs("attr_key", sink)
	assert.NoError(t, bpr.ConsumeLogs(t.Context(), inBatch))
	outBatches := sink.AllLogs()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitLogsReturnError(t *testing.T) {
	inBatch := plog.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1")

	err := errors.New("test_error")
	bpr := NewBatchPerResourceLogs("attr_key", consumertest.NewErr(err))
	assert.Equal(t, err, bpr.ConsumeLogs(t.Context(), inBatch))
}

func TestSplitLogsSameResource(t *testing.T) {
	inBatch := plog.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "same_attr_val", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "same_attr_val", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "same_attr_val", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "same_attr_val", "1")
	expected := plog.NewLogs()
	inBatch.CopyTo(expected)

	sink := new(consumertest.LogsSink)
	bpr := NewBatchPerResourceLogs("same_attr_val", sink)
	assert.NoError(t, bpr.ConsumeLogs(t.Context(), inBatch))
	outBatches := sink.AllLogs()
	require.Len(t, outBatches, 1)
	assert.Equal(t, expected, outBatches[0])
}

func TestSplitLogsIntoDifferentBatches(t *testing.T) {
	inBatch := plog.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "2")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "3")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "4")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "2")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "3")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "4")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "diff_attr_key", "1")
	expected := plog.NewLogs()
	inBatch.CopyTo(expected)

	sink := new(consumertest.LogsSink)
	bpr := NewBatchPerResourceLogs("attr_key", sink)
	assert.NoError(t, bpr.ConsumeLogs(t.Context(), inBatch))
	outBatches := sink.AllLogs()
	require.Len(t, outBatches, 5)
	sortLogs(outBatches, "attr_key")
	assert.Equal(t, newLogs(expected.ResourceLogs().At(8)), outBatches[0])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(0), expected.ResourceLogs().At(4)), outBatches[1])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(1), expected.ResourceLogs().At(5)), outBatches[2])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(2), expected.ResourceLogs().At(6)), outBatches[3])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(3), expected.ResourceLogs().At(7)), outBatches[4])
}

func TestSplitLogsIntoDifferentBatchesWithMultipleKeys(t *testing.T) {
	inBatch := plog.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1", "attr_key2", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "2", "attr_key2", "2")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "3", "attr_key2", "3")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "4", "attr_key2", "4")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "1", "attr_key2", "1")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "2", "attr_key2", "2")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "3", "attr_key2", "3")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "4", "attr_key2", "4")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "5", "attr_key2", "6")
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "diff_attr_key", "1")
	expected := plog.NewLogs()
	inBatch.CopyTo(expected)

	sink := new(consumertest.LogsSink)
	bpr := NewMultiBatchPerResourceLogs([]string{"attr_key", "attr_key2"}, sink)
	assert.NoError(t, bpr.ConsumeLogs(t.Context(), inBatch))
	outBatches := sink.AllLogs()
	require.Len(t, outBatches, 6)
	sortLogs(outBatches, "attr_key")
	assert.Equal(t, newLogs(expected.ResourceLogs().At(9)), outBatches[0])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(0), expected.ResourceLogs().At(4)), outBatches[1])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(1), expected.ResourceLogs().At(5)), outBatches[2])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(2), expected.ResourceLogs().At(6)), outBatches[3])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(3), expected.ResourceLogs().At(7)), outBatches[4])
	assert.Equal(t, newLogs(expected.ResourceLogs().At(8)), outBatches[5])
}

func newTraces(rss ...ptrace.ResourceSpans) ptrace.Traces {
	td := ptrace.NewTraces()
	for _, rs := range rss {
		rs.CopyTo(td.ResourceSpans().AppendEmpty())
	}
	return td
}

func sortTraces(tds []ptrace.Traces, attrKey string) {
	sort.Slice(tds, func(i, j int) bool {
		valI := ""
		if av, ok := tds[i].ResourceSpans().At(0).Resource().Attributes().Get(attrKey); ok {
			valI = av.Str()
		}
		valJ := ""
		if av, ok := tds[j].ResourceSpans().At(0).Resource().Attributes().Get(attrKey); ok {
			valJ = av.Str()
		}
		return valI < valJ
	})
}

func fillResourceSpans(rs ptrace.ResourceSpans, kv ...string) {
	for i := 0; i < len(kv); i += 2 {
		rs.Resource().Attributes().PutStr(kv[i], kv[i+1])
	}

	rs.Resource().Attributes().PutInt("__other_key__", 123)
	ils := rs.ScopeSpans().AppendEmpty()
	firstSpan := ils.Spans().AppendEmpty()
	firstSpan.SetName("first-span")
	firstSpan.SetTraceID(pcommon.TraceID([16]byte{byte(rand.Int())}))
	secondSpan := ils.Spans().AppendEmpty()
	secondSpan.SetName("second-span")
	secondSpan.SetTraceID(pcommon.TraceID([16]byte{byte(rand.Int())}))
}

func newMetrics(rms ...pmetric.ResourceMetrics) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, rm := range rms {
		rm.CopyTo(md.ResourceMetrics().AppendEmpty())
	}
	return md
}

func sortMetrics(tds []pmetric.Metrics, attrKey string) {
	sort.Slice(tds, func(i, j int) bool {
		valI := ""
		if av, ok := tds[i].ResourceMetrics().At(0).Resource().Attributes().Get(attrKey); ok {
			valI = av.Str()
		}
		valJ := ""
		if av, ok := tds[j].ResourceMetrics().At(0).Resource().Attributes().Get(attrKey); ok {
			valJ = av.Str()
		}
		return valI < valJ
	})
}

func fillResourceMetrics(rs pmetric.ResourceMetrics, kv ...string) {
	for i := 0; i < len(kv); i += 2 {
		rs.Resource().Attributes().PutStr(kv[i], kv[i+1])
	}
	rs.Resource().Attributes().PutInt("__other_key__", 123)
	ils := rs.ScopeMetrics().AppendEmpty()
	firstMetric := ils.Metrics().AppendEmpty()
	firstMetric.SetName("first-metric")
	firstMetric.SetEmptyGauge()
	secondMetric := ils.Metrics().AppendEmpty()
	secondMetric.SetName("second-metric")
	secondMetric.SetEmptySum()
}

func newLogs(rls ...plog.ResourceLogs) plog.Logs {
	ld := plog.NewLogs()
	for _, rl := range rls {
		rl.CopyTo(ld.ResourceLogs().AppendEmpty())
	}
	return ld
}

func sortLogs(tds []plog.Logs, attrKey string) {
	sort.Slice(tds, func(i, j int) bool {
		valI := ""
		if av, ok := tds[i].ResourceLogs().At(0).Resource().Attributes().Get(attrKey); ok {
			valI = av.Str()
		}
		valJ := ""
		if av, ok := tds[j].ResourceLogs().At(0).Resource().Attributes().Get(attrKey); ok {
			valJ = av.Str()
		}
		return valI < valJ
	})
}

func fillResourceLogs(rs plog.ResourceLogs, kv ...string) {
	for i := 0; i < len(kv); i += 2 {
		rs.Resource().Attributes().PutStr(kv[i], kv[i+1])
	}
	rs.Resource().Attributes().PutInt("__other_key__", 123)
	ils := rs.ScopeLogs().AppendEmpty()
	firstLogRecord := ils.LogRecords().AppendEmpty()
	firstLogRecord.SetFlags(plog.LogRecordFlags(rand.Int32()))
	secondLogRecord := ils.LogRecords().AppendEmpty()
	secondLogRecord.SetFlags(plog.LogRecordFlags(rand.Int32()))
}

// ctxTracesSink records the context passed to each ConsumeTraces call.
type ctxTracesSink struct {
	contexts []context.Context
	consumertest.TracesSink
}

func (*ctxTracesSink) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }
func (s *ctxTracesSink) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	s.contexts = append(s.contexts, ctx)
	return s.TracesSink.ConsumeTraces(ctx, td)
}

// ctxMetricsSink records the context passed to each ConsumeMetrics call.
type ctxMetricsSink struct {
	contexts []context.Context
	consumertest.MetricsSink
}

func (*ctxMetricsSink) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }
func (s *ctxMetricsSink) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	s.contexts = append(s.contexts, ctx)
	return s.MetricsSink.ConsumeMetrics(ctx, md)
}

// ctxLogsSink records the context passed to each ConsumeLogs call.
type ctxLogsSink struct {
	contexts []context.Context
	consumertest.LogsSink
}

func (*ctxLogsSink) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }
func (s *ctxLogsSink) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	s.contexts = append(s.contexts, ctx)
	return s.LogsSink.ConsumeLogs(ctx, ld)
}

func TestWithMetadataInjectionTraces(t *testing.T) {
	t.Run("single resource injects metadata", func(t *testing.T) {
		inBatch := ptrace.NewTraces()
		fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "val1")

		sink := &ctxTracesSink{}
		bpr := NewBatchPerResourceTraces("attr_key", sink, WithMetadataInjection())
		require.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))

		require.Len(t, sink.contexts, 1)
		meta := client.FromContext(sink.contexts[0]).Metadata
		assert.Equal(t, []string{"val1"}, meta.Get("attr_key"))
	})

	t.Run("multiple resources same value injects shared metadata", func(t *testing.T) {
		inBatch := ptrace.NewTraces()
		fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "shared")
		fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "shared")

		sink := &ctxTracesSink{}
		bpr := NewBatchPerResourceTraces("attr_key", sink, WithMetadataInjection())
		require.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))

		require.Len(t, sink.contexts, 1)
		assert.Equal(t, []string{"shared"}, client.FromContext(sink.contexts[0]).Metadata.Get("attr_key"))
	})

	t.Run("multiple resources different values each batch gets own metadata", func(t *testing.T) {
		inBatch := ptrace.NewTraces()
		fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "a")
		fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "b")

		sink := &ctxTracesSink{}
		bpr := NewBatchPerResourceTraces("attr_key", sink, WithMetadataInjection())
		require.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))

		require.Len(t, sink.contexts, 2)
		vals := []string{
			client.FromContext(sink.contexts[0]).Metadata.Get("attr_key")[0],
			client.FromContext(sink.contexts[1]).Metadata.Get("attr_key")[0],
		}
		assert.ElementsMatch(t, []string{"a", "b"}, vals)
	})

	t.Run("no metadata when option not set", func(t *testing.T) {
		inBatch := ptrace.NewTraces()
		fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", "val1")

		sink := &ctxTracesSink{}
		bpr := NewBatchPerResourceTraces("attr_key", sink)
		require.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))

		require.Len(t, sink.contexts, 1)
		assert.Empty(t, client.FromContext(sink.contexts[0]).Metadata.Get("attr_key"))
	})

	t.Run("key absent from resource omitted from metadata", func(t *testing.T) {
		inBatch := ptrace.NewTraces()
		fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "other_key", "val")

		sink := &ctxTracesSink{}
		bpr := NewBatchPerResourceTraces("attr_key", sink, WithMetadataInjection())
		require.NoError(t, bpr.ConsumeTraces(t.Context(), inBatch))

		require.Len(t, sink.contexts, 1)
		assert.Empty(t, client.FromContext(sink.contexts[0]).Metadata.Get("attr_key"))
	})
}

func TestWithMetadataInjectionMetrics(t *testing.T) {
	t.Run("single resource injects metadata", func(t *testing.T) {
		inBatch := pmetric.NewMetrics()
		fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "val1")

		sink := &ctxMetricsSink{}
		bpr := NewBatchPerResourceMetrics("attr_key", sink, WithMetadataInjection())
		require.NoError(t, bpr.ConsumeMetrics(t.Context(), inBatch))

		require.Len(t, sink.contexts, 1)
		assert.Equal(t, []string{"val1"}, client.FromContext(sink.contexts[0]).Metadata.Get("attr_key"))
	})

	t.Run("multiple resources different values each batch gets own metadata", func(t *testing.T) {
		inBatch := pmetric.NewMetrics()
		fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "a")
		fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", "b")

		sink := &ctxMetricsSink{}
		bpr := NewBatchPerResourceMetrics("attr_key", sink, WithMetadataInjection())
		require.NoError(t, bpr.ConsumeMetrics(t.Context(), inBatch))

		require.Len(t, sink.contexts, 2)
		vals := []string{
			client.FromContext(sink.contexts[0]).Metadata.Get("attr_key")[0],
			client.FromContext(sink.contexts[1]).Metadata.Get("attr_key")[0],
		}
		assert.ElementsMatch(t, []string{"a", "b"}, vals)
	})
}

func TestWithMetadataInjectionLogs(t *testing.T) {
	t.Run("single resource injects metadata", func(t *testing.T) {
		inBatch := plog.NewLogs()
		fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "val1")

		sink := &ctxLogsSink{}
		bpr := NewBatchPerResourceLogs("attr_key", sink, WithMetadataInjection())
		require.NoError(t, bpr.ConsumeLogs(t.Context(), inBatch))

		require.Len(t, sink.contexts, 1)
		assert.Equal(t, []string{"val1"}, client.FromContext(sink.contexts[0]).Metadata.Get("attr_key"))
	})

	t.Run("multiple resources different values each batch gets own metadata", func(t *testing.T) {
		inBatch := plog.NewLogs()
		fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "a")
		fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", "b")

		sink := &ctxLogsSink{}
		bpr := NewBatchPerResourceLogs("attr_key", sink, WithMetadataInjection())
		require.NoError(t, bpr.ConsumeLogs(t.Context(), inBatch))

		require.Len(t, sink.contexts, 2)
		vals := []string{
			client.FromContext(sink.contexts[0]).Metadata.Get("attr_key")[0],
			client.FromContext(sink.contexts[1]).Metadata.Get("attr_key")[0],
		}
		assert.ElementsMatch(t, []string{"a", "b"}, vals)
	})
}

func BenchmarkBatchPerResourceTraces(b *testing.B) {
	inBatch := ptrace.NewTraces()
	rss := inBatch.ResourceSpans()
	rss.EnsureCapacity(64)
	for i := range 64 {
		fillResourceSpans(rss.AppendEmpty(), "attr_key", strconv.Itoa(i%8))
	}
	bpr := NewBatchPerResourceTraces("attr_key", consumertest.NewNop())
	b.ReportAllocs()

	for b.Loop() {
		if err := bpr.ConsumeTraces(b.Context(), inBatch); err != nil {
			b.Fail()
		}
	}
}

func BenchmarkBatchPerResourceMetrics(b *testing.B) {
	inBatch := pmetric.NewMetrics()
	inBatch.ResourceMetrics().EnsureCapacity(64)
	for i := range 64 {
		fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", strconv.Itoa(i%8))
	}
	bpr := NewBatchPerResourceMetrics("attr_key", consumertest.NewNop())
	b.ReportAllocs()

	for b.Loop() {
		if err := bpr.ConsumeMetrics(b.Context(), inBatch); err != nil {
			b.Fail()
		}
	}
}

func BenchmarkBatchPerResourceLogs(b *testing.B) {
	inBatch := plog.NewLogs()
	inBatch.ResourceLogs().EnsureCapacity(64)
	for i := range 64 {
		fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", strconv.Itoa(i%8))
	}
	bpr := NewBatchPerResourceLogs("attr_key", consumertest.NewNop())
	b.ReportAllocs()

	for b.Loop() {
		if err := bpr.ConsumeLogs(b.Context(), inBatch); err != nil {
			b.Fail()
		}
	}
}
