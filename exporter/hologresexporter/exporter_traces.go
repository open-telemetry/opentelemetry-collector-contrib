// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"context"
	"database/sql"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesExporter struct {
	logger *zap.Logger
	cfg    *Config
	db     *sql.DB
}

func newTracesExporter(logger *zap.Logger, cfg *Config) *tracesExporter {
	return &tracesExporter{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *tracesExporter) start(ctx context.Context, _ component.Host) error {
	db, err := openDB(e.cfg.DSN)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		if err := createTracesTable(ctx, e.db, e.cfg.TracesTableName, e.cfg.TTL); err != nil {
			return err
		}
	}
	return nil
}

func (e *tracesExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *tracesExporter) pushTraceData(_ context.Context, _ ptrace.Traces) error {
	return nil
}
