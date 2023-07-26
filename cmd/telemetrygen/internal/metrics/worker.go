// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type worker struct {
	running        *atomic.Bool    // pointer to shared flag that indicates it's time to stop the test
	numMetrics     int             // how many metrics the worker has to generate (only when duration==0)
	totalDuration  time.Duration   // how long to run the test for (overrides `numMetrics`)
	limitPerSecond rate.Limit      // how many metrics per second to generate
	wg             *sync.WaitGroup // notify when done
	logger         *zap.Logger     // logger
	index          int             // worker index
}

func (w worker) simulateMetrics(res *resource.Resource, exporter sdkmetric.Exporter) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int64

	for w.running.Load() {
		rm := metricdata.ResourceMetrics{
			Resource: res,
			ScopeMetrics: []metricdata.ScopeMetrics{
				{
					Metrics: []metricdata.Metrics{
						{
							Name: "gen",
							Data: metricdata.Gauge[int64]{
								DataPoints: []metricdata.DataPoint[int64]{
									{
										Time:  time.Now(),
										Value: i,
									},
								},
							},
						},
					},
				},
			},
		}
		if err := exporter.Export(context.Background(), &rm); err != nil {
			w.logger.Fatal("exporter failed", zap.Error(err))
		}
		if err := limiter.Wait(context.Background()); err != nil {
			w.logger.Fatal("limiter wait failed, retry", zap.Error(err))
		}

		i++
		if w.numMetrics != 0 && i >= int64(w.numMetrics) {
			break
		}
	}

	w.logger.Info("metrics generated", zap.Int64("metrics", i))
	w.wg.Done()
}
