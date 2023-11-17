// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Start starts the metric telemetry generator
func Start(cfg *Config) error {
	logger, err := common.CreateLogger(cfg.SkipSettingGRPCLogger)
	if err != nil {
		return err
	}
	logger.Info("starting the metrics generator with configuration", zap.Any("config", cfg))

	expFunc := func() (sdkmetric.Exporter, error) {
		var exp sdkmetric.Exporter
		if cfg.UseHTTP {
			logger.Info("starting HTTP exporter")
			exp, err = otlpmetrichttp.New(context.Background(), httpExporterOptions(cfg)...)
		} else {
			logger.Info("starting gRPC exporter")
			exp, err = otlpmetricgrpc.New(context.Background(), grpcExporterOptions(cfg)...)
		}
		return exp, err
	}

	if err = Run(cfg, expFunc, logger); err != nil {
		logger.Error("failed to stop the exporter", zap.Error(err))
		return err
	}

	return nil
}

// Run executes the test scenario.
func Run(c *Config, exp func() (sdkmetric.Exporter, error), logger *zap.Logger) error {
	if c.TotalDuration > 0 {
		c.NumMetrics = 0
	} else if c.NumMetrics <= 0 {
		return fmt.Errorf("either `metrics` or `duration` must be greater than 0")
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of metrics isn't being throttled")
	} else {
		logger.Info("generation of metrics is limited", zap.Float64("per-second", float64(limit)))
	}

	wg := sync.WaitGroup{}
	res := resource.NewWithAttributes(semconv.SchemaURL, c.GetAttributes()...)

	running := &atomic.Bool{}
	running.Store(true)

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numMetrics:     c.NumMetrics,
			metricType:     c.MetricType,
			limitPerSecond: limit,
			totalDuration:  c.TotalDuration,
			running:        running,
			wg:             &wg,
			logger:         logger.With(zap.Int("worker", i)),
			index:          i,
		}

		go w.simulateMetrics(res, exp, c.GetTelemetryAttributes())
	}
	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	return nil
}
