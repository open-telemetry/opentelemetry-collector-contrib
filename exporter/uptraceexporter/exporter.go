// Copyright 2021 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package uptraceexporter

import (
	"context"

	"github.com/uptrace/uptrace-go/spanexp"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type traceExporter struct {
	cfg    *Config
	logger *zap.Logger
	upexp  *spanexp.Exporter
}

func newTraceExporter(cfg *Config, logger *zap.Logger) (*traceExporter, error) {
	if cfg.HTTPClientSettings.Endpoint != "" {
		logger.Warn("uptraceexporter: endpoint is not supported; use dsn instead")
	}

	client, err := cfg.HTTPClientSettings.ToClient()
	if err != nil {
		return nil, err
	}

	upexp, err := spanexp.NewExporter(&spanexp.Config{
		DSN:        cfg.DSN,
		HTTPClient: client,
		MaxRetries: -1, // disable retries because Collector already handles them
	})
	if err != nil {
		return nil, err
	}

	exporter := &traceExporter{
		cfg:    cfg,
		logger: logger,
		upexp:  upexp,
	}

	return exporter, nil
}

// pushTraceData is the method called when trace data is available.
func (e *traceExporter) pushTraceData(ctx context.Context, traces pdata.Traces) (int, error) {
	return 0, nil
}

func (e *traceExporter) Shutdown(ctx context.Context) error {
	return e.upexp.Shutdown(ctx)
}
