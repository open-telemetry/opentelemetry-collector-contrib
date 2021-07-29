// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package batchperresourceattr

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestSplitTracesOneResourceSpans(t *testing.T) {
	inBatch := pdata.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.TracesSink)
	bpr := NewBatchPerResourceTraces("attr_key", sink)
	assert.NoError(t, bpr.ConsumeTraces(context.Background(), inBatch))
	outBatches := sink.AllTraces()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitTracesReturnError(t *testing.T) {
	inBatch := pdata.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))

	err := errors.New("test_error")
	bpr := NewBatchPerResourceTraces("attr_key", consumertest.NewErr(err))
	assert.Equal(t, err, bpr.ConsumeTraces(context.Background(), inBatch))
}

func TestSplitTracesSameResource(t *testing.T) {
	inBatch := pdata.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.TracesSink)
	bpr := NewBatchPerResourceTraces("same_attr_val", sink)
	assert.NoError(t, bpr.ConsumeTraces(context.Background(), inBatch))
	outBatches := sink.AllTraces()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitTracesIntoDifferentBatches(t *testing.T) {
	inBatch := pdata.NewTraces()
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("2"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("3"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("4"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("2"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("3"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("4"))
	fillResourceSpans(inBatch.ResourceSpans().AppendEmpty(), "diff_attr_key", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.TracesSink)
	bpr := NewBatchPerResourceTraces("attr_key", sink)
	assert.NoError(t, bpr.ConsumeTraces(context.Background(), inBatch))
	outBatches := sink.AllTraces()
	require.Len(t, outBatches, 5)
	sortTraces(outBatches, "attr_key")
	assert.Equal(t, newTraces(inBatch.ResourceSpans().At(8)), outBatches[0])
	assert.Equal(t, newTraces(inBatch.ResourceSpans().At(0), inBatch.ResourceSpans().At(4)), outBatches[1])
	assert.Equal(t, newTraces(inBatch.ResourceSpans().At(1), inBatch.ResourceSpans().At(5)), outBatches[2])
	assert.Equal(t, newTraces(inBatch.ResourceSpans().At(2), inBatch.ResourceSpans().At(6)), outBatches[3])
	assert.Equal(t, newTraces(inBatch.ResourceSpans().At(3), inBatch.ResourceSpans().At(7)), outBatches[4])
}

func TestSplitMetricsOneResourceMetrics(t *testing.T) {
	inBatch := pdata.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.MetricsSink)
	bpr := NewBatchPerResourceMetrics("attr_key", sink)
	assert.NoError(t, bpr.ConsumeMetrics(context.Background(), inBatch))
	outBatches := sink.AllMetrics()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitMetricsReturnError(t *testing.T) {
	inBatch := pdata.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))

	err := errors.New("test_error")
	bpr := NewBatchPerResourceMetrics("attr_key", consumertest.NewErr(err))
	assert.Equal(t, err, bpr.ConsumeMetrics(context.Background(), inBatch))
}

func TestSplitMetricsSameResource(t *testing.T) {
	inBatch := pdata.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.MetricsSink)
	bpr := NewBatchPerResourceMetrics("same_attr_val", sink)
	assert.NoError(t, bpr.ConsumeMetrics(context.Background(), inBatch))
	outBatches := sink.AllMetrics()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitMetricsIntoDifferentBatches(t *testing.T) {
	inBatch := pdata.NewMetrics()
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("2"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("3"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("4"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("2"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("3"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("4"))
	fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "diff_attr_key", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.MetricsSink)
	bpr := NewBatchPerResourceMetrics("attr_key", sink)
	assert.NoError(t, bpr.ConsumeMetrics(context.Background(), inBatch))
	outBatches := sink.AllMetrics()
	require.Len(t, outBatches, 5)
	sortMetrics(outBatches, "attr_key")
	assert.Equal(t, newMetrics(inBatch.ResourceMetrics().At(8)), outBatches[0])
	assert.Equal(t, newMetrics(inBatch.ResourceMetrics().At(0), inBatch.ResourceMetrics().At(4)), outBatches[1])
	assert.Equal(t, newMetrics(inBatch.ResourceMetrics().At(1), inBatch.ResourceMetrics().At(5)), outBatches[2])
	assert.Equal(t, newMetrics(inBatch.ResourceMetrics().At(2), inBatch.ResourceMetrics().At(6)), outBatches[3])
	assert.Equal(t, newMetrics(inBatch.ResourceMetrics().At(3), inBatch.ResourceMetrics().At(7)), outBatches[4])
}

func TestSplitLogsOneResourceLogs(t *testing.T) {
	inBatch := pdata.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.LogsSink)
	bpr := NewBatchPerResourceLogs("attr_key", sink)
	assert.NoError(t, bpr.ConsumeLogs(context.Background(), inBatch))
	outBatches := sink.AllLogs()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitLogsReturnError(t *testing.T) {
	inBatch := pdata.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))

	err := errors.New("test_error")
	bpr := NewBatchPerResourceLogs("attr_key", consumertest.NewErr(err))
	assert.Equal(t, err, bpr.ConsumeLogs(context.Background(), inBatch))
}

func TestSplitLogsSameResource(t *testing.T) {
	inBatch := pdata.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "same_attr_val", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.LogsSink)
	bpr := NewBatchPerResourceLogs("same_attr_val", sink)
	assert.NoError(t, bpr.ConsumeLogs(context.Background(), inBatch))
	outBatches := sink.AllLogs()
	require.Len(t, outBatches, 1)
	assert.Equal(t, inBatch, outBatches[0])
}

func TestSplitLogsIntoDifferentBatches(t *testing.T) {
	inBatch := pdata.NewLogs()
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("2"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("3"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("4"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("1"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("2"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("3"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString("4"))
	fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "diff_attr_key", pdata.NewAttributeValueString("1"))

	sink := new(consumertest.LogsSink)
	bpr := NewBatchPerResourceLogs("attr_key", sink)
	assert.NoError(t, bpr.ConsumeLogs(context.Background(), inBatch))
	outBatches := sink.AllLogs()
	require.Len(t, outBatches, 5)
	sortLogs(outBatches, "attr_key")
	assert.Equal(t, newLogs(inBatch.ResourceLogs().At(8)), outBatches[0])
	assert.Equal(t, newLogs(inBatch.ResourceLogs().At(0), inBatch.ResourceLogs().At(4)), outBatches[1])
	assert.Equal(t, newLogs(inBatch.ResourceLogs().At(1), inBatch.ResourceLogs().At(5)), outBatches[2])
	assert.Equal(t, newLogs(inBatch.ResourceLogs().At(2), inBatch.ResourceLogs().At(6)), outBatches[3])
	assert.Equal(t, newLogs(inBatch.ResourceLogs().At(3), inBatch.ResourceLogs().At(7)), outBatches[4])
}

func newTraces(rss ...pdata.ResourceSpans) pdata.Traces {
	td := pdata.NewTraces()
	for _, rs := range rss {
		rs.CopyTo(td.ResourceSpans().AppendEmpty())
	}
	return td
}

func sortTraces(tds []pdata.Traces, attrKey string) {
	sort.Slice(tds, func(i, j int) bool {
		valI := ""
		if av, ok := tds[i].ResourceSpans().At(0).Resource().Attributes().Get(attrKey); ok {
			valI = av.StringVal()
		}
		valJ := ""
		if av, ok := tds[j].ResourceSpans().At(0).Resource().Attributes().Get(attrKey); ok {
			valJ = av.StringVal()
		}
		return valI < valJ
	})
}

func fillResourceSpans(rs pdata.ResourceSpans, key string, val pdata.AttributeValue) {
	rs.Resource().Attributes().Upsert(key, val)
	rs.Resource().Attributes().Upsert("__other_key__", pdata.NewAttributeValueInt(123))
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()
	firstSpan := ils.Spans().AppendEmpty()
	firstSpan.SetName("first-span")
	firstSpan.SetTraceID(pdata.NewTraceID([16]byte{byte(rand.Int())}))
	secondSpan := ils.Spans().AppendEmpty()
	secondSpan.SetName("second-span")
	secondSpan.SetTraceID(pdata.NewTraceID([16]byte{byte(rand.Int())}))
}

func newMetrics(rms ...pdata.ResourceMetrics) pdata.Metrics {
	md := pdata.NewMetrics()
	for _, rm := range rms {
		rm.CopyTo(md.ResourceMetrics().AppendEmpty())
	}
	return md
}

func sortMetrics(tds []pdata.Metrics, attrKey string) {
	sort.Slice(tds, func(i, j int) bool {
		valI := ""
		if av, ok := tds[i].ResourceMetrics().At(0).Resource().Attributes().Get(attrKey); ok {
			valI = av.StringVal()
		}
		valJ := ""
		if av, ok := tds[j].ResourceMetrics().At(0).Resource().Attributes().Get(attrKey); ok {
			valJ = av.StringVal()
		}
		return valI < valJ
	})
}

func fillResourceMetrics(rs pdata.ResourceMetrics, key string, val pdata.AttributeValue) {
	rs.Resource().Attributes().Upsert(key, val)
	rs.Resource().Attributes().Upsert("__other_key__", pdata.NewAttributeValueInt(123))
	ils := rs.InstrumentationLibraryMetrics().AppendEmpty()
	firstMetric := ils.Metrics().AppendEmpty()
	firstMetric.SetName("first-metric")
	firstMetric.SetDataType(pdata.MetricDataType(rand.Int() % 4))
	secondMetric := ils.Metrics().AppendEmpty()
	secondMetric.SetName("second-metric")
	secondMetric.SetDataType(pdata.MetricDataType(rand.Int() % 4))
}

func newLogs(rls ...pdata.ResourceLogs) pdata.Logs {
	ld := pdata.NewLogs()
	for _, rl := range rls {
		rl.CopyTo(ld.ResourceLogs().AppendEmpty())
	}
	return ld
}

func sortLogs(tds []pdata.Logs, attrKey string) {
	sort.Slice(tds, func(i, j int) bool {
		valI := ""
		if av, ok := tds[i].ResourceLogs().At(0).Resource().Attributes().Get(attrKey); ok {
			valI = av.StringVal()
		}
		valJ := ""
		if av, ok := tds[j].ResourceLogs().At(0).Resource().Attributes().Get(attrKey); ok {
			valJ = av.StringVal()
		}
		return valI < valJ
	})
}

func fillResourceLogs(rs pdata.ResourceLogs, key string, val pdata.AttributeValue) {
	rs.Resource().Attributes().Upsert(key, val)
	rs.Resource().Attributes().Upsert("__other_key__", pdata.NewAttributeValueInt(123))
	ils := rs.InstrumentationLibraryLogs().AppendEmpty()
	firstLogRecord := ils.Logs().AppendEmpty()
	firstLogRecord.SetName("first-log-record")
	firstLogRecord.SetFlags(rand.Uint32())
	secondLogRecord := ils.Logs().AppendEmpty()
	secondLogRecord.SetName("second-log-record")
	secondLogRecord.SetFlags(rand.Uint32())
}

func BenchmarkBatchPerResourceTraces(b *testing.B) {
	inBatch := pdata.NewTraces()
	rss := inBatch.ResourceSpans()
	rss.EnsureCapacity(64)
	for i := 0; i < 64; i++ {
		fillResourceSpans(rss.AppendEmpty(), "attr_key", pdata.NewAttributeValueString(strconv.Itoa(i%8)))
	}
	bpr := NewBatchPerResourceTraces("attr_key", consumertest.NewNop())
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if err := bpr.ConsumeTraces(context.Background(), inBatch); err != nil {
			b.Fail()
		}
	}
}

func BenchmarkBatchPerResourceMetrics(b *testing.B) {
	inBatch := pdata.NewMetrics()
	inBatch.ResourceMetrics().EnsureCapacity(64)
	for i := 0; i < 64; i++ {
		fillResourceMetrics(inBatch.ResourceMetrics().AppendEmpty(), "attr_key", pdata.NewAttributeValueString(strconv.Itoa(i%8)))
	}
	bpr := NewBatchPerResourceMetrics("attr_key", consumertest.NewNop())
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if err := bpr.ConsumeMetrics(context.Background(), inBatch); err != nil {
			b.Fail()
		}
	}
}

func BenchmarkBatchPerResourceLogs(b *testing.B) {
	inBatch := pdata.NewLogs()
	inBatch.ResourceLogs().EnsureCapacity(64)
	for i := 0; i < 64; i++ {
		fillResourceLogs(inBatch.ResourceLogs().AppendEmpty(), "attr_key", pdata.NewAttributeValueString(strconv.Itoa(i%8)))
	}
	bpr := NewBatchPerResourceLogs("attr_key", consumertest.NewNop())
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if err := bpr.ConsumeLogs(context.Background(), inBatch); err != nil {
			b.Fail()
		}
	}
}
