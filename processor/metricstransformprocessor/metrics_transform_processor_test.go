// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
)

func TestMetricsTransformProcessor(t *testing.T) {
	for _, test := range standardTests {
		t.Run(test.name, func(t *testing.T) {
			next := new(consumertest.MetricsSink)

			p := &metricsTransformProcessor{
				transforms: test.transforms,
				logger:     zap.NewExample(),
			}

			mtp, err := processorhelper.NewMetricsProcessor(
				context.Background(),
				processortest.NewNopCreateSettings(),
				&Config{},
				next,
				p.processMetrics,
				processorhelper.WithCapabilities(consumerCapabilities))
			require.NoError(t, err)

			caps := mtp.Capabilities()
			assert.Equal(t, true, caps.MutatesData)

			// process
			inMetrics := pmetric.NewMetrics()
			inMetricsSlice := inMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
			for _, m := range test.in {
				m.MoveTo(inMetricsSlice.AppendEmpty())
			}
			cErr := mtp.ConsumeMetrics(context.Background(), inMetrics)
			assert.NoError(t, cErr)

			// get and check results
			got := next.AllMetrics()
			require.Equal(t, 1, len(got))
			gotMetricsSlice := pmetric.NewMetricSlice()
			if got[0].ResourceMetrics().Len() > 0 {
				gotMetricsSlice = got[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			}

			require.Equal(t, len(test.out), gotMetricsSlice.Len())
			for i, expected := range test.out {
				assert.Equal(t, sortDataPoints(expected), sortDataPoints(gotMetricsSlice.At(i)))
			}
		})
	}
}

func sortDataPoints(m pmetric.Metric) pmetric.Metric {
	switch m.Type() {
	case pmetric.MetricTypeSum:
		m.Sum().DataPoints().Sort(lessNumberDatapoint)
	case pmetric.MetricTypeGauge:
		m.Gauge().DataPoints().Sort(lessNumberDatapoint)
	case pmetric.MetricTypeHistogram:
		m.Histogram().DataPoints().Sort(lessHistogramDatapoint)
	}
	return m
}

func lessNumberDatapoint(a, b pmetric.NumberDataPoint) bool {
	if a.StartTimestamp() != b.StartTimestamp() {
		return a.StartTimestamp() < b.StartTimestamp()
	}
	if a.Timestamp() != b.Timestamp() {
		return a.Timestamp() < b.Timestamp()
	}
	if a.IntValue() != b.IntValue() {
		return a.IntValue() < b.IntValue()
	}
	if a.DoubleValue() != b.DoubleValue() {
		return a.DoubleValue() < b.DoubleValue()
	}
	return lessAttributes(a.Attributes(), b.Attributes())
}

func lessHistogramDatapoint(a, b pmetric.HistogramDataPoint) bool {
	if a.StartTimestamp() != b.StartTimestamp() {
		return a.StartTimestamp() < b.StartTimestamp()
	}
	if a.Timestamp() != b.Timestamp() {
		return a.Timestamp() < b.Timestamp()
	}
	if a.Count() != b.Count() {
		return a.Count() < b.Count()
	}
	if a.Sum() != b.Sum() {
		return a.Sum() < b.Sum()
	}
	return lessAttributes(a.Attributes(), b.Attributes())
}

func lessAttributes(a, b pcommon.Map) bool {
	if a.Len() != b.Len() {
		return a.Len() < b.Len()
	}

	var res bool
	a.Range(func(k string, v pcommon.Value) bool {
		bv, ok := b.Get(k)
		if !ok || v.Str() < bv.Str() {
			res = true
			return false
		}
		return true
	})
	return res
}
