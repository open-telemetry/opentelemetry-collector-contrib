package postgresexporter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
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

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	e.logger.Info("Received logs", zap.Any("logs", ld))
	start := time.Now()

	err := doWithTx(ctx, e.client, func(tx *sql.Tx) error {
		statement, err := tx.PrepareContext(ctx, e.insertSQL)
		if err != nil {
			return fmt.Errorf("failed to prepare insert statement: %w", err)
		}
		defer statement.Close()

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
				scopeLogs := logs.ScopeLogs().At(j)
				rs := scopeLogs.LogRecords()
				scopeURL := scopeLogs.SchemaUrl()
				scopeName := scopeLogs.Scope().Name()
				scopeVersion := scopeLogs.Scope().Version()
				scopeAttr := attributesToMap(scopeLogs.Scope().Attributes())

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
						return fmt.Errorf("failed to execute insert statement: %w", err)
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
	return strings.ReplaceAll(createLogsTableSQL, "{{TABLE_NAME}}", cfg.LogsTableName)
}

func renderInsertLogsSQL(cfg *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplate, cfg.LogsTableName)
}

const (
	createLogsTableSQL = `
    CREATE TABLE IF NOT EXISTS {{TABLE_NAME}} (
        "Timestamp" TIMESTAMP(9) NOT NULL,
        "TimestampTime" TIMESTAMP NOT NULL DEFAULT NOW(),
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
        
        PRIMARY KEY ("ServiceName", "TimestampTime")
    );

    CREATE INDEX IF NOT EXISTS idx_trace_id ON {{TABLE_NAME}} ("TraceId");
    CREATE INDEX IF NOT EXISTS idx_res_attr_key ON {{TABLE_NAME}} (("ResourceAttributes"->>'key'));
    CREATE INDEX IF NOT EXISTS idx_res_attr_value ON {{TABLE_NAME}} (("ResourceAttributes"->>'value'));
    CREATE INDEX IF NOT EXISTS idx_scope_attr_key ON {{TABLE_NAME}} (("ScopeAttributes"->>'key'));
    CREATE INDEX IF NOT EXISTS idx_scope_attr_value ON {{TABLE_NAME}} (("ScopeAttributes"->>'value'));
    CREATE INDEX IF NOT EXISTS idx_log_attr_key ON {{TABLE_NAME}} (("LogAttributes"->>'key'));
    CREATE INDEX IF NOT EXISTS idx_log_attr_value ON {{TABLE_NAME}} (("LogAttributes"->>'value'));
    CREATE INDEX IF NOT EXISTS idx_body ON {{TABLE_NAME}} ("Body");
`

	insertLogsSQLTemplate = `INSERT INTO %s (
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
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)`
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

func attributesToMap(attributes pcommon.Map) string {
	m := make(map[string]string, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		m[k] = v.AsString()
		return true
	})

	jsonData, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}

	return string(jsonData)
}
