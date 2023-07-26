// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

type exporter interface {
	export(plog.Logs) error
}

type gRPCClientExporter struct {
	client plogotlp.GRPCClient
}

func (e *gRPCClientExporter) export(logs plog.Logs) error {
	req := plogotlp.NewExportRequestFromLogs(logs)
	if _, err := e.client.Export(context.Background(), req); err != nil {
		return err
	}
	return nil
}

// Start starts the log telemetry generator
func Start(cfg *Config) error {
	logger, err := common.CreateLogger()
	if err != nil {
		return err
	}

	if cfg.UseHTTP {
		return fmt.Errorf("http is not supported by 'telemetrygen logs'")
	}

	if !cfg.Insecure {
		return fmt.Errorf("'telemetrygen logs' only supports insecure gRPC")
	}

	// only support grpc in insecure mode
	clientConn, err := grpc.DialContext(context.TODO(), cfg.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	exporter := &gRPCClientExporter{
		client: plogotlp.NewGRPCClient(clientConn),
	}

	if err = Run(cfg, exporter, logger); err != nil {
		logger.Error("failed to stop the exporter", zap.Error(err))
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

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numLogs:        c.NumLogs,
			limitPerSecond: limit,
			totalDuration:  c.TotalDuration,
			running:        running,
			wg:             &wg,
			logger:         logger.With(zap.Int("worker", i)),
			index:          i,
		}

		go w.simulateLogs(res, exp)
	}
	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	return nil
}
