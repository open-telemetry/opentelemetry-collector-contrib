// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/attributes"
)

func newBenchmarkTranslator(b *testing.B, logger *zap.Logger, opts ...Option) *Translator {
	options := append([]Option{
		WithFallbackSourceProvider(testProvider("fallbackHostname")),
		WithHistogramMode(HistogramModeDistributions),
		WithNumberMode(NumberModeCumulativeToDelta),
	}, opts...)

	tr, err := New(
		logger,
		options...,
	)

	require.NoError(b, err)
	return tr
}

// createBenchmarkGaugeMetrics creates n Gauge data points.
func createBenchmarkGaugeMetrics(n int, additionalAttributes map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	attrs := rm.Resource().Attributes()
	attrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	for attr, val := range additionalAttributes {
		attrs.InsertString(attr, val)
	}
	ilms := rm.ScopeMetrics()

	ilm := ilms.AppendEmpty()
	metricsArray := ilm.Metrics()
	metricsArray.AppendEmpty() // first one is TypeNone to test that it's ignored

	for i := 0; i < n; i++ {
		// IntGauge
		met := metricsArray.AppendEmpty()
		met.SetName(fmt.Sprintf("int.gauge.%d", i))
		met.SetDataType(pmetric.MetricDataTypeGauge)
		dpsInt := met.Gauge().DataPoints()
		dpInt := dpsInt.AppendEmpty()
		dpInt.SetTimestamp(seconds(0))
		dpInt.SetIntVal(1)
	}

	return md
}

// createBenchmarkDeltaExponentialHistogramMetrics creates n ExponentialHistogram data points, each with b buckets
// in each store, with a delta aggregation temporality.
func createBenchmarkDeltaExponentialHistogramMetrics(n int, b int, additionalAttributes map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	for attr, val := range additionalAttributes {
		resourceAttrs.InsertString(attr, val)
	}

	ilms := rm.ScopeMetrics()
	ilm := ilms.AppendEmpty()
	metricsArray := ilm.Metrics()

	for i := 0; i < n; i++ {
		met := metricsArray.AppendEmpty()
		met.SetName("expHist.test")
		met.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
		met.ExponentialHistogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
		points := met.ExponentialHistogram().DataPoints()
		point := points.AppendEmpty()

		point.SetScale(6)

		point.SetCount(30)
		point.SetZeroCount(10)
		point.SetSum(math.Pi)

		buckets := make([]uint64, b)
		for i := 0; i < b; i++ {
			buckets[i] = 10
		}
		immutableBuckets := pcommon.NewImmutableUInt64Slice(buckets)

		point.Negative().SetOffset(2)
		point.Negative().SetBucketCounts(immutableBuckets)

		point.Positive().SetOffset(3)
		point.Positive().SetBucketCounts(immutableBuckets)

		point.SetTimestamp(seconds(0))
	}

	return md
}

// createBenchmarkDeltaSumMetrics creates n Sum data points with a delta aggregation temporality.
func createBenchmarkDeltaSumMetrics(n int, additionalAttributes map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	attrs := rm.Resource().Attributes()
	attrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	for attr, val := range additionalAttributes {
		attrs.InsertString(attr, val)
	}
	ilms := rm.ScopeMetrics()

	ilm := ilms.AppendEmpty()
	metricsArray := ilm.Metrics()
	metricsArray.AppendEmpty() // first one is TypeNone to test that it's ignored

	for i := 0; i < n; i++ {
		met := metricsArray.AppendEmpty()
		met.SetName("double.delta.monotonic.sum")
		met.SetDataType(pmetric.MetricDataTypeSum)
		met.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
		dpsDouble := met.Sum().DataPoints()
		dpDouble := dpsDouble.AppendEmpty()
		dpDouble.SetTimestamp(seconds(0))
		dpDouble.SetDoubleVal(math.E)
	}

	return md
}

func benchmarkMapMetrics(metrics pmetric.Metrics, b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		tr := newBenchmarkTranslator(b, zap.NewNop())
		consumer := &mockFullConsumer{}
		err := tr.MapMetrics(ctx, metrics, consumer)
		assert.NoError(b, err)
	}
}

func BenchmarkMapDeltaExponentialHistogramMetrics1_5(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(1, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10_5(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(10, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics100_5(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(100, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1000_5(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(1000, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10000_5(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(10000, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1_50(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(1, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10_50(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(10, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics100_50(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(100, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1000_50(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(1000, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10000_50(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(10000, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1_500(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(1, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10_500(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(10, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics100_500(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(100, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1000_500(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(1000, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10000_500(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(10000, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1_5000(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(1, 5000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10_5000(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(10, 5000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics100_5000(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(100, 5000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1000_5000(b *testing.B) {
	metrics := createBenchmarkDeltaExponentialHistogramMetrics(1000, 5000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics10(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(10, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics100(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(100, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics1000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(1000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics10000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(10000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics100000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(100000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics1000000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(1000000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics10000000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(10000000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics10(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(10, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics100(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(100, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics1000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(1000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics10000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(10000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics100000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(100000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics1000000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(1000000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics10000000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(10000000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}
