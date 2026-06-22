// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/log"
)

// Start starts the profile telemetry generator
func Start(cfg *Config) error {
	logger, err := log.CreateLogger(cfg.SkipSettingGRPCLogger)
	if err != nil {
		return err
	}

	logger.Info("starting the profiles generator with configuration", zap.Any("config", cfg))

	if err := run(cfg, exporterFactory(cfg, logger), logger); err != nil {
		return err
	}

	return nil
}

type exporterFunc func() (profileExporter, error)

func exporterFactory(cfg *Config, logger *zap.Logger) exporterFunc {
	return func() (profileExporter, error) {
		if cfg.UseHTTP {
			logger.Info("starting HTTP exporter")
			return newHTTPExporter(cfg)
		}
		logger.Info("starting gRPC exporter")
		return newGRPCExporter(cfg)
	}
}

func run(c *Config, expF exporterFunc, logger *zap.Logger) error {
	if err := c.Validate(); err != nil {
		return err
	}

	if c.TotalDuration.Duration() > 0 || c.TotalDuration.IsInf() {
		c.NumProfiles = 0
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of profiles isn't being throttled")
	} else {
		logger.Info("generation of profiles is limited", zap.Float64("per-second", float64(limit)))
	}

	wg := sync.WaitGroup{}

	res := pcommon.NewMap()
	for _, attr := range c.GetAttributes() {
		setMapAttribute(res, attr)
	}

	running := &atomic.Bool{}
	running.Store(true)

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numProfiles:     c.NumProfiles,
			sampleCount:     c.SampleCount,
			stackDepth:      c.StackDepth,
			uniqueFunctions: c.UniqueFunctions,
			profileDuration: c.ProfileDuration,
			limitPerSecond:  limit,
			totalDuration:   c.TotalDuration,
			running:         running,
			wg:              &wg,
			logger:          logger.With(zap.Int("worker", i)),
			index:           i,
			traceID:         c.TraceID,
			spanID:          c.SpanID,
			batch:           c.Batch,
			batchSize:       c.BatchSize,
			loadSize:        c.LoadSize,
			allowFailures:   c.AllowExportFailures,
			rand:            rand.New(rand.NewPCG(uint64(time.Now().UnixNano()+int64(i)), 0)),
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
		go w.simulateProfiles(res, exp, c.GetTelemetryAttributes())
	}
	if c.TotalDuration.Duration() > 0 && !c.TotalDuration.IsInf() {
		time.Sleep(c.TotalDuration.Duration())
		running.Store(false)
	}
	wg.Wait()
	return nil
}
