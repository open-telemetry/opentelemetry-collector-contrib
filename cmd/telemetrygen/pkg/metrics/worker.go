// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

type worker struct {
	running                *atomic.Bool                 // pointer to shared flag that indicates it's time to stop the test
	metricName             string                       // name of metric to generate
	metricType             MetricType                   // type of metric to generate
	mixedMetrics           bool                         // if true, randomly select metric type
	aggregationTemporality AggregationTemporality       // Temporality type to use
	exemplars              []metricdata.Exemplar[int64] // exemplars to attach to the metric
	numMetrics             int                          // how many metrics the worker has to generate (only when duration==0)
	enforceUnique          bool                         // if true, the worker will generate unique timeseries
	totalDuration          types.DurationWithInf        // how long to run the test for (overrides `numMetrics`)
	limitPerSecond         rate.Limit                   // how many metrics per second to generate
	wg                     *sync.WaitGroup              // notify when done
	logger                 *zap.Logger                  // logger
	index                  int                          // worker index
	clock                  Clock                        // clock
  allowFailures          bool                         // whether to continue on export failures
	rand                   *rand.Rand                   // random number generator for mixed metrics
}

// Available metric types for random selection
var availableMetricTypes = []MetricType{
	MetricTypeGauge,
	MetricTypeSum,
	MetricTypeHistogram,
	MetricTypeExponentialHistogram,
}

// We use a 15-element bounds slice for histograms below, so there must be 16 buckets here.
// From metrics.proto:
// The number of elements in bucket_counts array must be by one greater than
// the number of elements in explicit_bounds array.
var histogramBucketSamples = []struct {
	bucketCounts []uint64
	sum          int64
}{
	{
		[]uint64{0, 0, 1, 0, 0, 0, 3, 4, 1, 1, 0, 0, 0, 0, 0, 0},
		3940,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 2, 4, 4, 0, 0, 0, 0, 0, 0, 0},
		4455,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 1, 4, 3, 2, 0, 0, 0, 0, 0, 0},
		5337,
	},
	{
		[]uint64{0, 0, 1, 0, 1, 0, 2, 2, 1, 3, 0, 0, 0, 0, 0, 0},
		4477,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 1, 3, 2, 2, 2, 0, 0, 0, 0, 0, 0},
		4670,
	},
	{
		[]uint64{0, 0, 0, 1, 1, 0, 1, 1, 1, 5, 0, 0, 0, 0, 0, 0},
		5670,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 2, 1, 1, 4, 2, 0, 0, 0, 0, 0, 0},
		5091,
	},
	{
		[]uint64{0, 0, 2, 0, 0, 0, 2, 4, 1, 1, 0, 0, 0, 0, 0, 0},
		3420,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 1, 3, 2, 4, 0, 0, 0, 0, 0, 0},
		5917,
	},
	{
		[]uint64{0, 0, 1, 0, 1, 0, 0, 4, 4, 0, 0, 0, 0, 0, 0, 0},
		3988,
	},
}

func (w worker) simulateMetrics(res *resource.Resource, exporter sdkmetric.Exporter, signalAttrs []attribute.KeyValue, tb *timeBox) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)

	startTime := w.clock.Now()

	var i int64
	for w.running.Load() {
		if w.enforceUnique {
			signalAttrs = append(signalAttrs, tb.getAttribute())
		}
		var metrics []metricdata.Metrics
		now := w.clock.Now()
		if w.aggregationTemporality.AsTemporality() == metricdata.DeltaTemporality {
			startTime = now.Add(-1 * time.Second)
		}

		// Select metric type - either fixed or random
		metricType := w.metricType
		if w.mixedMetrics {
			metricType = availableMetricTypes[w.rand.IntN(len(availableMetricTypes))]
		}

		switch metricType {
		case MetricTypeGauge:
			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.Gauge[int64]{
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Time:       now,
							Value:      i,
							Attributes: attribute.NewSet(signalAttrs...),
							Exemplars:  w.exemplars,
						},
					},
				},
			})
		case MetricTypeSum:
			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: w.aggregationTemporality.AsTemporality(),
					DataPoints: []metricdata.DataPoint[int64]{
						{
							StartTime:  startTime,
							Time:       now,
							Value:      i,
							Attributes: attribute.NewSet(signalAttrs...),
							Exemplars:  w.exemplars,
						},
					},
				},
			})
		case MetricTypeHistogram:
			var totalCount uint64
			iteration := uint64(i) % 10
			sum := histogramBucketSamples[iteration].sum
			bucketCounts := histogramBucketSamples[iteration].bucketCounts
			for _, count := range bucketCounts {
				totalCount += count
			}
			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.Histogram[int64]{
					Temporality: w.aggregationTemporality.AsTemporality(),
					DataPoints: []metricdata.HistogramDataPoint[int64]{
						{
							StartTime:  startTime,
							Time:       now,
							Attributes: attribute.NewSet(signalAttrs...),
							Exemplars:  w.exemplars,
							Count:      totalCount,
							Sum:        sum,
							// Bounds from https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/sdk.md#explicit-bucket-histogram-aggregation
							Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
							BucketCounts: bucketCounts,
						},
					},
				},
			})
		case MetricTypeExponentialHistogram:
			// Generate realistic exponential histogram data
			iteration := uint64(i) % 10
			sum := histogramBucketSamples[iteration].sum
			count := uint64(10 + w.rand.IntN(20)) // Random count between 10-30

			// Create exponential histogram with base-2 buckets
			maxBuckets := 8
			positiveBuckets := make([]uint64, maxBuckets)
			negativeBuckets := make([]uint64, maxBuckets)

			// Distribute counts across buckets using exponential distribution
			for j := 0; j < int(count); j++ {
				// Generate values with exponential distribution
				value := float64(w.rand.IntN(1000))
				if value > 0 {
					// Calculate bucket index for positive values
					bucket := int(math.Log2(value)) + 1
					if bucket >= 0 && bucket < maxBuckets {
						positiveBuckets[bucket]++
					}
				} else if value < 0 {
					// Calculate bucket index for negative values
					bucket := int(math.Log2(-value)) + 1
					if bucket >= 0 && bucket < maxBuckets {
						negativeBuckets[bucket]++
					}
				}
			}

			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.ExponentialHistogram[int64]{
					Temporality: w.aggregationTemporality.AsTemporality(),
					DataPoints: []metricdata.ExponentialHistogramDataPoint[int64]{
						{
							StartTime:     startTime,
							Time:          now,
							Attributes:    attribute.NewSet(signalAttrs...),
							Exemplars:     w.exemplars,
							Count:         count,
							Sum:           sum,
							Scale:         0,
							ZeroCount:     0,
							ZeroThreshold: 0.0,
							PositiveBucket: metricdata.ExponentialBucket{
								Offset: 0,
								Counts: positiveBuckets,
							},
							NegativeBucket: metricdata.ExponentialBucket{
								Offset: 0,
								Counts: negativeBuckets,
							},
						},
					},
				},
			})
		default:
			w.logger.Fatal("unknown metric type")
		}

		rm := metricdata.ResourceMetrics{
			Resource:     res,
			ScopeMetrics: []metricdata.ScopeMetrics{{Metrics: metrics}},
		}

		if err := limiter.Wait(context.Background()); err != nil {
			w.logger.Fatal("limiter wait failed, retry", zap.Error(err))
		}

		if err := exporter.Export(context.Background(), &rm); err != nil {
			if w.allowFailures {
				w.logger.Error("exporter failed, continuing due to --allow-export-failures", zap.Error(err))
			} else {
				w.logger.Fatal("exporter failed", zap.Error(err))
			}
		}

		i++
		if w.numMetrics != 0 && i >= int64(w.numMetrics) {
			break
		}
	}

	w.logger.Info("metrics generated", zap.Int64("metrics", i))
	w.wg.Done()
}
