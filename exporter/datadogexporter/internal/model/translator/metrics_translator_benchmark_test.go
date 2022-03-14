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
	"github.com/stretchr/testify/require"
	"math"
	"context"
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/attributes"
)

func newBenchmarkTranslator(b *testing.B, logger *zap.Logger, opts ...Option) *Translator {
	options := append([]Option{
		WithFallbackHostnameProvider(testProvider("fallbackHostname")),
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

func createBenchmarkGaugeMetrics(n int, additionalAttributes map[string]string, name, version string) pdata.Metrics {
	md := pdata.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	attrs := rm.Resource().Attributes()
	attrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	for attr, val := range additionalAttributes {
		attrs.InsertString(attr, val)
	}
	ilms := rm.InstrumentationLibraryMetrics()

	ilm := ilms.AppendEmpty()
	ilm.InstrumentationLibrary().SetName(name)
	ilm.InstrumentationLibrary().SetVersion(version)
	metricsArray := ilm.Metrics()
	metricsArray.AppendEmpty() // first one is TypeNone to test that it's ignored

	for i := 0; i < n; i ++ {
		// IntGauge
		met := metricsArray.AppendEmpty()
		met.SetName(fmt.Sprintf("int.gauge.%d", i))
		met.SetDataType(pdata.MetricDataTypeGauge)
		dpsInt := met.Gauge().DataPoints()
		dpInt := dpsInt.AppendEmpty()
		dpInt.SetTimestamp(seconds(0))
		dpInt.SetIntVal(1)
	}

	return md
}

func createBenchmarkExponentialHistogramMetrics(n int, t int, additionalAttributes map[string]string) pdata.Metrics {
	md := pdata.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	for attr, val := range additionalAttributes {
		resourceAttrs.InsertString(attr, val)
	}

	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.AppendEmpty()
	metricsArray := ilm.Metrics()

	for i := 0; i < n; i++ {
		met := metricsArray.AppendEmpty()
		met.SetName("expHist.test")
		met.SetDataType(pdata.MetricDataTypeExponentialHistogram)
		met.ExponentialHistogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
		points := met.ExponentialHistogram().DataPoints()
		point := points.AppendEmpty()

		point.SetScale(6)

		point.SetCount(30)
		point.SetZeroCount(10)
		point.SetSum(math.Pi)

		buckets := make([]uint64, t)
		for i := 0; i < t; i++ {
			buckets[i] = 10
		}

		point.Negative().SetOffset(2)
		point.Negative().SetBucketCounts(buckets)

		point.Positive().SetOffset(3)
		point.Positive().SetBucketCounts(buckets)

		point.SetTimestamp(seconds(0))
	}

	return md
}


func createBenchmarkDeltaSumMetrics(n int, additionalAttributes map[string]string, name, version string) pdata.Metrics {
	md := pdata.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	attrs := rm.Resource().Attributes()
	attrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	for attr, val := range additionalAttributes {
		attrs.InsertString(attr, val)
	}
	ilms := rm.InstrumentationLibraryMetrics()

	ilm := ilms.AppendEmpty()
	ilm.InstrumentationLibrary().SetName(name)
	ilm.InstrumentationLibrary().SetVersion(version)
	metricsArray := ilm.Metrics()
	metricsArray.AppendEmpty() // first one is TypeNone to test that it's ignored

	for i := 0; i < n; i ++ {
		met := metricsArray.AppendEmpty()
		met.SetName("double.delta.monotonic.sum")
		met.SetDataType(pdata.MetricDataTypeSum)
		met.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
		dpsDouble := met.Sum().DataPoints()
		dpDouble := dpsDouble.AppendEmpty()
		dpDouble.SetTimestamp(seconds(0))
		dpDouble.SetDoubleVal(math.E)
	}

	return md
}

func benchmarkMapMetrics(metrics pdata.Metrics, b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		tr := newBenchmarkTranslator(b, zap.NewNop())
		consumer := &mockFullConsumer{}
		tr.MapMetrics(ctx, metrics, consumer)
	}
}

func BenchmarkMapDeltaExponentialHistogramMetrics1_5(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(1, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10_5(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(10, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics100_5(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(100, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1000_5(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(1000, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10000_5(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(10000, 5, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1_50(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(1, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10_50(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(10, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics100_50(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(100, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1000_50(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(1000, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10000_50(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(10000, 50, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1_500(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(1, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10_500(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(10, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics100_500(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(100, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1000_500(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(1000, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10000_500(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(10000, 500, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1_5000(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(1, 5000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics10_5000(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(10, 5000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics100_5000(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(100, 5000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaExponentialHistogramMetrics1000_5000(b *testing.B) {
	metrics := createBenchmarkExponentialHistogramMetrics(1000, 5000, map[string]string{
		"attribute_tag": "attribute_value",
	})

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics10(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(10, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics100(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(100, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics1000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(1000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics10000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(10000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics100000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(100000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics1000000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(1000000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapGaugeMetrics10000000(b *testing.B) {
	metrics := createBenchmarkGaugeMetrics(10000000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics10(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(10, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics100(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(100, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics1000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(1000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics10000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(10000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics100000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(100000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics1000000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(1000000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}

func BenchmarkMapDeltaSumMetrics10000000(b *testing.B) {
	metrics := createBenchmarkDeltaSumMetrics(10000000, map[string]string{
		"attribute_tag": "attribute_value",
	}, "test-name", "test-version")

	benchmarkMapMetrics(metrics, b)
}