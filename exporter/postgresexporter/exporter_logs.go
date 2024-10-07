package postgresexporter

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
)

type logsExporter struct {
	client    *sql.DB
	insertSQL string
	logger    *zap.Logger
	cfg       *Config
}

func newLogsExporter(logger *zap.Logger, cfg *Config) (*logsExporter, error) {
	client, err := newPostgresClient(cfg)
	if err != nil {
		return nil, err
	}

	return &logsExporter{
		client:    client,
		insertSQL: renderInsertLogsSQL(cfg),
		logger:    logger,
		cfg:       cfg,
	}, nil
}

func (e *logsExporter) start(ctx context.Context, _ component.Host) error {
	return createLogsTable(ctx, e.cfg, e.client)
}

func (e *logsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *logsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	start := time.Now()
	err := doWithTx(ctx, e.client, func(tx *sql.Tx) error {
		statement, err := tx.PrepareContext(ctx, e.insertSQL)
		if err != nil {
			return fmt.Errorf("PrepareContext:%w", err)
		}
		defer func() {
			_ = statement.Close()
		}()
		var serviceName string

		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			logs := ld.ResourceLogs().At(i)
			res := logs.Resource()
			resURL := logs.SchemaUrl()
			resAttr := attributesToMap(res.Attributes())
			if v, ok := res.Attributes().Get(conventions.AttributeServiceName); ok {
				serviceName = v.Str()
			}

			for j := 0; j < logs.ScopeLogs().Len(); j++ {
				rs := logs.ScopeLogs().At(j).LogRecords()
				scopeURL := logs.ScopeLogs().At(j).SchemaUrl()
				scopeName := logs.ScopeLogs().At(j).Scope().Name()
				scopeVersion := logs.ScopeLogs().At(j).Scope().Version()
				scopeAttr := attributesToMap(logs.ScopeLogs().At(j).Scope().Attributes())

				for k := 0; k < rs.Len(); k++ {
					r := rs.At(k)

					timestamp := r.Timestamp()
					if timestamp == 0 {
						timestamp = r.ObservedTimestamp()
					}

					logAttr := attributesToMap(r.Attributes())
					_, err = statement.ExecContext(ctx,
						timestamp.AsTime(),
						traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
						traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
						uint32(r.Flags()),
						r.SeverityText(),
						int32(r.SeverityNumber()),
						serviceName,
						r.Body().AsString(),
						resURL,
						resAttr,
						scopeURL,
						scopeName,
						scopeVersion,
						scopeAttr,
						logAttr,
					)
					if err != nil {
						return fmt.Errorf("ExecContext:%w", err)
					}
				}
			}
		}
		return nil
	})
	duration := time.Since(start)
	e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()),
		zap.String("cost", duration.String()))
	return err
}

func newPostgresClient(cfg *Config) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database)
	db, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		return nil, err
	}
	return db, nil
}

// DB functions
func createLogsTable(ctx context.Context, cfg *Config, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, renderCreateLogsTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create logs table sql: %w", err)
	}
	return nil
}

// SQL rendering functions below
func renderCreateLogsTableSQL(cfg *Config) string {
	return fmt.Sprintf(createLogsTableSQL, cfg.LogsTableName)
}

func renderInsertLogsSQL(cfg *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplate, cfg.LogsTableName)
}

const (
	createLogsTableSQL = `
		CREATE TABLE IF NOT EXISTS %s (
		"Timestamp" TIMESTAMP(9) NOT NULL,
		"TimestampTime" TIMESTAMP DEFAULT "Timestamp",
		"TraceId" TEXT,
		"SpanId" TEXT,
		"TraceFlags" SMALLINT,
		"SeverityText" TEXT,
		"SeverityNumber" SMALLINT,
		"ServiceName" TEXT,
		"Body" TEXT,
		"ResourceSchemaUrl" TEXT,
		"ResourceAttributes" JSONB,
		"ScopeSchemaUrl" TEXT,
		"ScopeName" TEXT,
		"ScopeVersion" TEXT,
		"ScopeAttributes" JSONB,
		"LogAttributes" JSONB,
		
		PRIMARY KEY ("ServiceName", "TimestampTime"),
		INDEX idx_trace_id ("TraceId"),
		INDEX idx_res_attr_key (("ResourceAttributes"->>'key')),
		INDEX idx_res_attr_value (("ResourceAttributes"->>'value')),
		INDEX idx_scope_attr_key (("ScopeAttributes"->>'key')),
		INDEX idx_scope_attr_value (("ScopeAttributes"->>'value')),
		INDEX idx_log_attr_key (("LogAttributes"->>'key')),
		INDEX idx_log_attr_value (("LogAttributes"->>'value')),
		INDEX idx_body ("Body")
	);		
	`

	insertLogsSQLTemplate = `
		INSERT INTO %s (
		"Timestamp",
		"TraceId",
		"SpanId",
		"TraceFlags",
		"SeverityText",
		"SeverityNumber",
		"ServiceName",
		"Body",
		"ResourceSchemaUrl",
		"ResourceAttributes",
		"ScopeSchemaUrl",
		"ScopeName",
		"ScopeVersion",
		"ScopeAttributes",
		"LogAttributes"
	) VALUES (
		?,
		?,
		?,
		?,
		?,
		?,
		?,
		?,
		?,
		?::jsonb,
		?,
		?,
		?,
		?::jsonb,
		?::jsonb
	);
	`
)

func doWithTx(_ context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("db.Begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func attributesToMap(attributes pcommon.Map) map[string]string {
	m := make(map[string]string, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		m[k] = v.AsString()
		return true
	})
	return m
}
