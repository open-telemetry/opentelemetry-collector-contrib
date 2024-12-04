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
	metricType     metricType                   // type of metric to generate
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
		[]uint64{0, 0, 0, 0, 0},
		0,
	},

	{
		[]uint64{0, 1, 0, 0, 0},
		1,
	},
	{
		[]uint64{0, 1, 0, 1, 0},
		4,
	},
	{
		[]uint64{1, 1, 1, 0, 0},
		3,
	},
	{
		[]uint64{0, 0, 1, 2, 1},
		12,
	},
	{
		[]uint64{0, 0, 0, 0, 0},
		0,
	},
	{
		[]uint64{0, 1, 0, 0, 0},
		1,
	},
	{
		[]uint64{0, 2, 0, 0, 0},
		2,
	},
	{
		[]uint64{1, 0, 1, 1, 0},
		5,
	},
	{
		[]uint64{2, 0, 1, 0, 1},
		6,
	},
}

func (w worker) simulateMetrics(res *resource.Resource, exporterFunc func() (sdkmetric.Exporter, error), signalAttrs []attribute.KeyValue) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)

	exporter, err := exporterFunc()
	if err != nil {
		w.logger.Error("failed to create the exporter", zap.Error(err))
		return
	}

	defer func() {
		w.logger.Info("stopping the exporter")
		if tempError := exporter.Shutdown(context.Background()); tempError != nil {
			w.logger.Error("failed to stop the exporter", zap.Error(tempError))
		}
	}()

	var i int64
	for w.running.Load() {
		var metrics []metricdata.Metrics

		switch w.metricType {
		case metricTypeGauge:
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
		case metricTypeSum:
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
		case metricTypeHistogram:
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
							// Simple bounds assumption
							Bounds:       []float64{0, 1, 2, 3, 4},
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
