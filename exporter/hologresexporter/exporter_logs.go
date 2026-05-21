// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"context"
	"database/sql"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsExporter struct {
	logger *zap.Logger
	cfg    *Config
	db     *sql.DB
}

func newLogsExporter(logger *zap.Logger, cfg *Config) *logsExporter {
	return &logsExporter{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *logsExporter) start(ctx context.Context, _ component.Host) error {
	db, err := openDB(e.cfg.DSN)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		if err := createLogsTable(ctx, e.db, e.cfg.LogsTableName, e.cfg.TTL); err != nil {
			return err
		}
	}
	return nil
}

func (e *logsExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *logsExporter) pushLogData(_ context.Context, _ plog.Logs) error {
	return nil
}
