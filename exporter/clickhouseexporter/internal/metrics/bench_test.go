// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func init() {
	SetLogger(zap.NewNop())
}

// makeExemplars creates a realistic ExemplarSlice with n exemplars, each
// having filtered attributes, timestamps, values, and trace/span IDs.
func makeExemplars(n int) pmetric.ExemplarSlice {
	exemplars := pmetric.NewExemplarSlice()
	timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range n {
		e := exemplars.AppendEmpty()
		e.SetTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(time.Duration(i) * time.Second)))
		e.SetDoubleValue(float64(i) * 1.5)
		e.FilteredAttributes().PutStr("key", "value")
		e.FilteredAttributes().PutStr("key2", "value2")
		e.SetSpanID([8]byte{1, 2, 3, byte(i)})
		e.SetTraceID([16]byte{1, 2, 3, byte(i)})
	}
	return exemplars
}

func BenchmarkConvertExemplars(b *testing.B) {
	for _, n := range []int{1, 3, 10} {
		exemplars := makeExemplars(n)
		b.Run(fmt.Sprintf("exemplars=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				convertExemplars(exemplars)
			}
		})
	}
}

func BenchmarkConvertSliceToArraySet(b *testing.B) {
	for _, n := range []int{5, 20, 100} {
		slice := make([]uint64, n)
		for i := range slice {
			slice[i] = uint64(i)
		}
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				convertSliceToArraySet(slice)
			}
		})
	}
}

func BenchmarkConvertValueAtQuantile(b *testing.B) {
	for _, n := range []int{1, 5, 10} {
		quantiles := pmetric.NewSummaryDataPointValueAtQuantileSlice()
		for i := range n {
			q := quantiles.AppendEmpty()
			q.SetQuantile(float64(i) * 0.1)
			q.SetValue(float64(i) * 10.0)
		}
		b.Run(fmt.Sprintf("quantiles=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				convertValueAtQuantile(quantiles)
			}
		})
	}
}

// BenchmarkMetricsBatchPrep benchmarks the full data preparation path for
// gauge metrics including AttributesToMap, GetServiceName, and convertExemplars.
func BenchmarkMetricsBatchPrep(b *testing.B) {
	for _, count := range []int{100, 1000} {
		metrics := makeGaugeMetrics(count)
		b.Run(fmt.Sprintf("datapoints=%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				for _, model := range metrics {
					resAttr := AttributesToMap(model.metadata.ResAttr)
					scopeAttr := AttributesToMap(model.metadata.ScopeInstr.Attributes())
					serviceName := GetServiceName(model.metadata.ResAttr)
					_ = resAttr
					_ = scopeAttr
					_ = serviceName
					for i := 0; i < model.gauge.DataPoints().Len(); i++ {
						dp := model.gauge.DataPoints().At(i)
						_ = AttributesToMap(dp.Attributes())
						convertExemplars(dp.Exemplars())
						getValue(dp.IntValue(), dp.DoubleValue(), dp.ValueType())
					}
				}
			}
		})
	}
}

func makeGaugeMetrics(count int) []*gaugeModel {
	rm := pmetric.NewResourceMetrics()
	rm.Resource().Attributes().PutStr("service.name", "bench-service")
	rm.Resource().Attributes().PutStr("host.name", "bench-host")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("bench-scope")
	sm.Scope().SetVersion("1.0.0")
	sm.Scope().Attributes().PutStr("lib", "clickhouse")
	timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	m := sm.Metrics().AppendEmpty()
	m.SetName("bench.gauge")
	m.SetUnit("count")
	m.SetDescription("benchmark gauge metric")
	gauge := m.SetEmptyGauge()
	for i := range count {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.Attributes().PutStr("label1", "value1")
		dp.Attributes().PutStr("label2", "value2")
		e := dp.Exemplars().AppendEmpty()
		e.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		e.SetIntValue(int64(i))
		e.FilteredAttributes().PutStr("key", "value")
		e.SetSpanID([8]byte{1, 2, 3, byte(i)})
		e.SetTraceID([16]byte{1, 2, 3, byte(i)})
	}

	return []*gaugeModel{
		{
			metricName:        m.Name(),
			metricDescription: m.Description(),
			metricUnit:        m.Unit(),
			metadata: &MetricsMetaData{
				ResAttr:    rm.Resource().Attributes(),
				ResURL:     "https://opentelemetry.io/schemas/1.4.0",
				ScopeURL:   "https://opentelemetry.io/schemas/1.7.0",
				ScopeInstr: sm.Scope(),
			},
			gauge: gauge,
		},
	}
}
