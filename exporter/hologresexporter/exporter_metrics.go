// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"context"
	"database/sql"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsExporter struct {
	logger *zap.Logger
	cfg    *Config
	db     *sql.DB
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) *metricsExporter {
	return &metricsExporter{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *metricsExporter) start(ctx context.Context, _ component.Host) error {
	db, err := openDB(e.cfg.DSN)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		if err := createMetricsTables(ctx, e.db, e.cfg.MetricsTableName, e.cfg.TTL); err != nil {
			return err
		}
	}
	return nil
}

func (e *metricsExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *metricsExporter) pushMetricData(_ context.Context, _ pmetric.Metrics) error {
	return nil
}
