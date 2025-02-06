// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type worker struct {
	running        *atomic.Bool                 // pointer to shared flag that indicates it's time to stop the test
	metricName     string                       // name of metric to generate
	metricType     MetricType                   // type of metric to generate
	exemplars      []metricdata.Exemplar[int64] // exemplars to attach to the metric
	numMetrics     int                          // how many metrics the worker has to generate (only when duration==0)
	totalDuration  time.Duration                // how long to run the test for (overrides `numMetrics`)
	limitPerSecond rate.Limit                   // how many metrics per second to generate
	wg             *sync.WaitGroup              // notify when done
	logger         *zap.Logger                  // logger
	index          int                          // worker index
}

var histogramBucketSamples = []struct {
	bucketCounts []uint64
	sum          int64
}{
	{
		[]uint64{0, 0, 1, 0, 0, 0, 3, 4, 1, 1, 0, 0, 0, 0, 0},
		3940,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 2, 4, 4, 0, 0, 0, 0, 0, 0},
		4455,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 1, 4, 3, 2, 0, 0, 0, 0, 0},
		5337,
	},
	{
		[]uint64{0, 0, 1, 0, 1, 0, 2, 2, 1, 3, 0, 0, 0, 0, 0},
		4477,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 1, 3, 2, 2, 2, 0, 0, 0, 0, 0},
		4670,
	},
	{
		[]uint64{0, 0, 0, 1, 1, 0, 1, 1, 1, 5, 0, 0, 0, 0, 0},
		5670,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 2, 1, 1, 4, 2, 0, 0, 0, 0, 0},
		5091,
	},
	{
		[]uint64{0, 0, 2, 0, 0, 0, 2, 4, 1, 1, 0, 0, 0, 0, 0},
		3420,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 1, 3, 2, 4, 0, 0, 0, 0, 0},
		5917,
	},
	{
		[]uint64{0, 0, 1, 0, 1, 0, 0, 4, 4, 0, 0, 0, 0, 0, 0},
		3988,
	},
}

func (w worker) simulateMetrics(res *resource.Resource, exporter sdkmetric.Exporter, signalAttrs []attribute.KeyValue) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)

	var i int64
	for w.running.Load() {
		var metrics []metricdata.Metrics

		switch w.metricType {
		case MetricTypeGauge:
			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.Gauge[int64]{
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Time:       time.Now(),
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
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.DataPoint[int64]{
						{
							StartTime:  time.Now().Add(-1 * time.Second),
							Time:       time.Now(),
							Value:      i,
							Attributes: attribute.NewSet(signalAttrs...),
							Exemplars:  w.exemplars,
						},
					},
				},
			})
		case MetricTypeHistogram:
			iteration := uint64(i) % 10
			sum := histogramBucketSamples[iteration].sum
			bucketCounts := histogramBucketSamples[iteration].bucketCounts
			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.Histogram[int64]{
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.HistogramDataPoint[int64]{
						{
							StartTime:  time.Now().Add(-1 * time.Second),
							Time:       time.Now(),
							Attributes: attribute.NewSet(signalAttrs...),
							Exemplars:  w.exemplars,
							Count:      iteration,
							Sum:        sum,
							// Bounds from https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/sdk.md#explicit-bucket-histogram-aggregation
							Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
							BucketCounts: bucketCounts,
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
			w.logger.Fatal("exporter failed", zap.Error(err))
		}

		i++
		if w.numMetrics != 0 && i >= int64(w.numMetrics) {
			break
		}
	}

	w.logger.Info("metrics generated", zap.Int64("metrics", i))
	w.wg.Done()
}
