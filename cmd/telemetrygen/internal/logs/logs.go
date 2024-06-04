// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
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

	e, err := newExporter(cfg)
	if err != nil {
		return err
	}

	if err = cfg.Validate(); err != nil {
		logger.Error("failed to validate the parameters for the test scenario.", zap.Error(err))
		return err
	}

	if err = Run(cfg, e, logger); err != nil {
		logger.Error("failed to execute the test scenario.", zap.Error(err))
		return err
	}

	return nil
}

// Run executes the test scenario.
func Run(c *Config, exp exporter, logger *zap.Logger) error {
	if c.TotalDuration > 0 {
		c.NumLogs = 0
	} else if c.NumLogs <= 0 {
		return fmt.Errorf("either `logs` or `duration` must be greater than 0")
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

		go w.simulateLogs(res, exp, c.GetTelemetryAttributes())
	}
	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	return nil
}

func parseSeverity(severityText string, severityNumber int32) (string, plog.SeverityNumber, error) {
	// severityNumber must range in [1,24]
	if severityNumber <= 0 || severityNumber >= 25 {
		return "", 0, fmt.Errorf("severity-number is out of range, the valid range is [1,24]")
	}

	sn := plog.SeverityNumber(severityNumber)

	// severity number should match well-known severityText
	switch severityText {
	case plog.SeverityNumberTrace.String():
		if !(severityNumber >= 1 && severityNumber <= 4) {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [1,4]", severityText, severityNumber)
		}
	case plog.SeverityNumberDebug.String():
		if !(severityNumber >= 5 && severityNumber <= 8) {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [5,8]", severityText, severityNumber)
		}
	case plog.SeverityNumberInfo.String():
		if !(severityNumber >= 9 && severityNumber <= 12) {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [9,12]", severityText, severityNumber)
		}
	case plog.SeverityNumberWarn.String():
		if !(severityNumber >= 13 && severityNumber <= 16) {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [13,16]", severityText, severityNumber)
		}
	case plog.SeverityNumberError.String():
		if !(severityNumber >= 17 && severityNumber <= 20) {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [17,20]", severityText, severityNumber)
		}
	case plog.SeverityNumberFatal.String():
		if !(severityNumber >= 21 && severityNumber <= 24) {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [21,24]", severityText, severityNumber)
		}
	}

	return severityText, sn, nil
}
