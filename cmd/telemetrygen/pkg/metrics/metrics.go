// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

	if err = run(cfg, exporterFactory(cfg, logger), logger); err != nil {
		return err
	}

	return nil
}

// run executes the test scenario.
func run(c *Config, expF exporterFunc, logger *zap.Logger) error {
	if err := c.Validate(); err != nil {
		return err
	}

	if c.TotalDuration > 0 {
		c.NumMetrics = 0
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
			numMetrics:             c.NumMetrics,
			metricName:             c.MetricName,
			metricType:             c.MetricType,
			aggregationTemporality: c.AggregationTemporality,
			exemplars:              exemplarsFromConfig(c),
			limitPerSecond:         limit,
			totalDuration:          c.TotalDuration,
			running:                running,
			wg:                     &wg,
			logger:                 logger.With(zap.Int("worker", i)),
			index:                  i,
			clock:                  &realClock{},
		}
		exp, err := expF()
		if err != nil {
			w.logger.Error("failed to create the exporter", zap.Error(err))
			return err
		}

		defer func() {
			w.logger.Info("stopping the exporter")
			if tempError := exp.Shutdown(context.Background()); tempError != nil {
				w.logger.Error("failed to stop the exporter", zap.Error(tempError))
			}
		}()

		go w.simulateMetrics(res, exp, c.GetTelemetryAttributes())
	}
	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	return nil
}

type exporterFunc func() (sdkmetric.Exporter, error)

func exporterFactory(cfg *Config, logger *zap.Logger) exporterFunc {
	return func() (sdkmetric.Exporter, error) {
		return createExporter(cfg, logger)
	}
}

func createExporter(cfg *Config, logger *zap.Logger) (sdkmetric.Exporter, error) {
	var exp sdkmetric.Exporter
	var err error
	if cfg.UseHTTP {
		var exporterOpts []otlpmetrichttp.Option

		logger.Info("starting HTTP exporter")
		exporterOpts, err = httpExporterOptions(cfg)
		if err != nil {
			return nil, err
		}
		exp, err = otlpmetrichttp.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP HTTP exporter: %w", err)
		}
	} else {
		var exporterOpts []otlpmetricgrpc.Option

		logger.Info("starting gRPC exporter")
		exporterOpts, err = grpcExporterOptions(cfg)
		if err != nil {
			return nil, err
		}
		exp, err = otlpmetricgrpc.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP gRPC exporter: %w", err)
		}
	}
	return exp, err
}

func exemplarsFromConfig(c *Config) []metricdata.Exemplar[int64] {
	if c.TraceID != "" || c.SpanID != "" {
		var exemplars []metricdata.Exemplar[int64]

		exemplar := metricdata.Exemplar[int64]{
			Value: 1,
			Time:  time.Now(),
		}

		if c.TraceID != "" {
			// we validated this already during the Validate() function for config
			traceID, _ := hex.DecodeString(c.TraceID)
			exemplar.TraceID = traceID
		}

		if c.SpanID != "" {
			// we validated this already during the Validate() function for config
			spanID, _ := hex.DecodeString(c.SpanID)
			exemplar.SpanID = spanID
		}

		return append(exemplars, exemplar)
	}
	return nil
}
