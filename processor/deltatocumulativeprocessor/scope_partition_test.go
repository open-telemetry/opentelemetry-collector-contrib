// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"
)

// Each source has ordered, non-overlapping observations. The only distinction
// between A and B is the selected instrumentation-scope identity field.
func TestScopePartitionPreservation(t *testing.T) {
	for _, field := range []string{"schema", "attribute"} {
		for _, kind := range []string{"sum", "histogram", "exponential_histogram"} {
			t.Run(field+"/"+kind, func(t *testing.T) {
				run := func(sources []string, values []uint64, times []pcommon.Timestamp) map[string][]string {
					ctx := t.Context()
					factory := NewFactory()
					cfg := factory.CreateDefaultConfig().(*Config)
					cfg.MaxStreams = 8
					sink := &consumertest.MetricsSink{}
					proc, err := factory.CreateMetrics(ctx, processortest.NewNopSettings(factory.Type()), cfg, sink)
					require.NoError(t, err)
					require.NoError(t, proc.Start(ctx, componenttest.NewNopHost()))
					defer func() { require.NoError(t, proc.Shutdown(ctx)) }()
					for i, source := range sources {
						md := pmetric.NewMetrics()
						sm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
						sm.Scope().SetName("partition-test")
						sm.Scope().SetVersion("1")
						if field == "schema" {
							if source == "A" {
								sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
							} else {
								sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
							}
						} else {
							sm.Scope().Attributes().PutStr("source", source)
						}
						m := sm.Metrics().AppendEmpty()
						m.SetName("partition.count")
						start := pcommon.Timestamp(1)
						if source == "A" && times[i] == 40 {
							start = 20
						}
						switch kind {
						case "sum":
							data := m.SetEmptySum()
							data.SetIsMonotonic(true)
							data.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
							dp := data.DataPoints().AppendEmpty()
							dp.SetStartTimestamp(start)
							dp.SetTimestamp(times[i])
							dp.SetIntValue(int64(values[i]))
						case "histogram":
							data := m.SetEmptyHistogram()
							data.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
							dp := data.DataPoints().AppendEmpty()
							dp.SetStartTimestamp(start)
							dp.SetTimestamp(times[i])
							dp.SetCount(values[i])
							dp.SetSum(float64(values[i]))
							dp.ExplicitBounds().FromRaw([]float64{1})
							dp.BucketCounts().FromRaw([]uint64{values[i], 0})
						case "exponential_histogram":
							data := m.SetEmptyExponentialHistogram()
							data.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
							dp := data.DataPoints().AppendEmpty()
							dp.SetStartTimestamp(start)
							dp.SetTimestamp(times[i])
							dp.SetCount(values[i])
							dp.SetSum(float64(values[i]))
							dp.Positive().BucketCounts().FromRaw([]uint64{values[i]})
						}
						require.NoError(t, proc.ConsumeMetrics(ctx, md))
					}
					result := map[string][]string{"A": {}, "B": {}}
					marshaler := &pmetric.JSONMarshaler{}
					for _, md := range sink.AllMetrics() {
						for i := 0; i < md.ResourceMetrics().Len(); i++ {
							rm := md.ResourceMetrics().At(i)
							for j := 0; j < rm.ScopeMetrics().Len(); j++ {
								sm := rm.ScopeMetrics().At(j)
								if sm.Metrics().Len() == 0 {
									continue
								}
								key := "A"
								if field == "schema" {
									if sm.SchemaUrl() == "https://opentelemetry.io/schemas/1.21.0" {
										key = "B"
									}
								} else {
									value, ok := sm.Scope().Attributes().Get("source")
									require.True(t, ok)
									key = value.Str()
								}
								one := pmetric.NewMetrics()
								sm.CopyTo(one.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty())
								raw, err := marshaler.MarshalMetrics(one)
								require.NoError(t, err)
								result[key] = append(result[key], string(raw))
							}
						}
					}
					return result
				}
				joint := run([]string{"A", "B", "A"}, []uint64{1, 3, 2}, []pcommon.Timestamp{20, 25, 40})
				a := run([]string{"A", "A"}, []uint64{1, 2}, []pcommon.Timestamp{20, 40})
				b := run([]string{"B"}, []uint64{3}, []pcommon.Timestamp{25})
				require.NotEmpty(t, a["A"], "the isolated witness must not be vacuous")
				require.Equal(t, a["A"], joint["A"], "source A must retain its isolated output")
				require.Equal(t, b["B"], joint["B"], "source B must retain its isolated output")
			})
		}
	}
}
