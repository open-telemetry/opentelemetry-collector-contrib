// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Start starts the log telemetry generator
func Start(cfg *Config) error {
	logger, err := common.CreateLogger(cfg.SkipSettingGRPCLogger)
	if err != nil {
		return err
	}

	logger.Info("starting the logs generator with configuration", zap.Any("config", cfg))

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
		c.NumLogs = 0
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of logs isn't being throttled")
	} else {
		logger.Info("generation of logs is limited", zap.Float64("per-second", float64(limit)))
	}

	wg := sync.WaitGroup{}
	res := resource.NewWithAttributes(semconv.SchemaURL, c.GetAttributes()...)

	running := &atomic.Bool{}
	running.Store(true)

	severityText, severityNumber, err := parseSeverity(c.SeverityText, c.SeverityNumber)
	if err != nil {
		return err
	}

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numLogs:        c.NumLogs,
			limitPerSecond: limit,
			body:           c.Body,
			severityText:   severityText,
			severityNumber: severityNumber,
			totalDuration:  c.TotalDuration,
			running:        running,
			wg:             &wg,
			logger:         logger.With(zap.Int("worker", i)),
			index:          i,
			traceID:        c.TraceID,
			spanID:         c.SpanID,
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
		go w.simulateLogs(res, exp, c.GetTelemetryAttributes())
	}
	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	return nil
}

type exporterFunc func() (sdklog.Exporter, error)

func exporterFactory(cfg *Config, logger *zap.Logger) exporterFunc {
	return func() (sdklog.Exporter, error) {
		return createExporter(cfg, logger)
	}
}

func createExporter(cfg *Config, logger *zap.Logger) (sdklog.Exporter, error) {
	var exp sdklog.Exporter
	var err error
	if cfg.UseHTTP {
		var exporterOpts []otlploghttp.Option

		logger.Info("starting HTTP exporter")
		exporterOpts, err = httpExporterOptions(cfg)
		if err != nil {
			return nil, err
		}
		exp, err = otlploghttp.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP HTTP exporter: %w", err)
		}
	} else {
		var exporterOpts []otlploggrpc.Option

		logger.Info("starting gRPC exporter")
		exporterOpts, err = grpcExporterOptions(cfg)
		if err != nil {
			return nil, err
		}
		exp, err = otlploggrpc.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP gRPC exporter: %w", err)
		}
	}
	return exp, err
}

func parseSeverity(severityText string, severityNumber int32) (string, log.Severity, error) {
	sn := log.Severity(severityNumber)
	if sn < log.SeverityTrace1 || sn > log.SeverityFatal4 {
		return "", log.SeverityUndefined, errors.New("severity-number is out of range, the valid range is [1,24]")
	}

	// severity number should match well-known severityText
	switch severityText {
	case plog.SeverityNumberTrace.String():
		if severityNumber < 1 || severityNumber > 4 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [1,4]", severityText, severityNumber)
		}
	case plog.SeverityNumberDebug.String():
		if severityNumber < 5 || severityNumber > 8 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [5,8]", severityText, severityNumber)
		}
	case plog.SeverityNumberInfo.String():
		if severityNumber < 9 || severityNumber > 12 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [9,12]", severityText, severityNumber)
		}
	case plog.SeverityNumberWarn.String():
		if severityNumber < 13 || severityNumber > 16 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [13,16]", severityText, severityNumber)
		}
	case plog.SeverityNumberError.String():
		if severityNumber < 17 || severityNumber > 20 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [17,20]", severityText, severityNumber)
		}
	case plog.SeverityNumberFatal.String():
		if severityNumber < 21 || severityNumber > 24 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [21,24]", severityText, severityNumber)
		}
	}

	return severityText, sn, nil
}
