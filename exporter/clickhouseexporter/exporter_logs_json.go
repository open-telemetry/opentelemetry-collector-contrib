// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/chjson"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"
)

type logsJSONExporter struct {
	db        driver.Conn
	insertSQL string

	resourceAttributesBufferPool *internal.ExporterStructPool[*chjson.JSONBuffer]
	scopeAttributesBufferPool    *internal.ExporterStructPool[*chjson.JSONBuffer]
	logAttributesBufferPool      *internal.ExporterStructPool[*chjson.JSONBuffer]
	traceHexBufferPool           *internal.ExporterStructPool[[]byte]
	spanHexBufferPool            *internal.ExporterStructPool[[]byte]

	logger *zap.Logger
	cfg    *Config
}

func newLogsJSONExporter(logger *zap.Logger, cfg *Config) (*logsJSONExporter, error) {
	numConsumers := cfg.QueueSettings.NumConsumers

	newJSONBuffer := func() (*chjson.JSONBuffer, error) {
		return chjson.NewJSONBuffer(2048, 256), nil
	}
	newHexBuffer := func() ([]byte, error) {
		return make([]byte, 0, 128), nil
	}

	resourceAttributesBufferPool, _ := internal.NewExporterStructPool[*chjson.JSONBuffer](numConsumers, newJSONBuffer)
	scopeAttributesBufferPool, _ := internal.NewExporterStructPool[*chjson.JSONBuffer](numConsumers, newJSONBuffer)
	logAttributesBufferPool, _ := internal.NewExporterStructPool[*chjson.JSONBuffer](numConsumers, newJSONBuffer)
	traceHexBufferPool, _ := internal.NewExporterStructPool[[]byte](numConsumers, newHexBuffer)
	spanHexBufferPool, _ := internal.NewExporterStructPool[[]byte](numConsumers, newHexBuffer)

	return &logsJSONExporter{
		insertSQL:                    renderInsertLogsJSONSQL(cfg),
		resourceAttributesBufferPool: resourceAttributesBufferPool,
		scopeAttributesBufferPool:    scopeAttributesBufferPool,
		logAttributesBufferPool:      logAttributesBufferPool,
		traceHexBufferPool:           traceHexBufferPool,
		spanHexBufferPool:            spanHexBufferPool,
		logger:                       logger,
		cfg:                          cfg,
	}, nil
}

func (e *logsJSONExporter) start(ctx context.Context, _ component.Host) error {
	dsn, err := e.cfg.buildDSN()
	if err != nil {
		return err
	}

	e.db, err = internal.NewClickhouseClient(dsn)
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		if err := internal.CreateDatabase(ctx, e.db, e.cfg.database(), e.cfg.clusterString()); err != nil {
			return err
		}

		if err := createLogsJSONTable(ctx, e.cfg, e.db); err != nil {
			return err
		}
	}

	return nil
}

func (e *logsJSONExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			e.logger.Warn("failed to close json logs db connection", zap.Error(err))
		}
	}

	e.resourceAttributesBufferPool.Destroy()
	e.scopeAttributesBufferPool.Destroy()
	e.logAttributesBufferPool.Destroy()
	e.traceHexBufferPool.Destroy()
	e.spanHexBufferPool.Destroy()

	return nil
}

func (e *logsJSONExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	batch, err := e.db.PrepareBatch(ctx, e.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		closeErr := batch.Close()
		if closeErr != nil {
			e.logger.Warn("failed to close json logs batch", zap.Error(closeErr))
		}
	}(batch)

	resourceAttributesBuffer := e.resourceAttributesBufferPool.Acquire()
	defer e.resourceAttributesBufferPool.Release(resourceAttributesBuffer)
	scopeAttributesBuffer := e.scopeAttributesBufferPool.Acquire()
	defer e.scopeAttributesBufferPool.Release(scopeAttributesBuffer)
	logAttributesBuffer := e.logAttributesBufferPool.Acquire()
	defer e.logAttributesBufferPool.Release(logAttributesBuffer)
	traceHexBuffer := e.traceHexBufferPool.Acquire()
	defer e.traceHexBufferPool.Release(traceHexBuffer)
	spanHexBuffer := e.spanHexBufferPool.Acquire()
	defer e.spanHexBufferPool.Release(spanHexBuffer)

	processStart := time.Now()

	var logCount int
	rsLogs := ld.ResourceLogs()
	rsLen := rsLogs.Len()
	for i := 0; i < rsLen; i++ {
		logs := rsLogs.At(i)
		res := logs.Resource()
		resURL := logs.SchemaUrl()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resourceAttributesBuffer.Reset()
		chjson.AttributesToJSON(resourceAttributesBuffer, resAttr)

		slLen := logs.ScopeLogs().Len()
		for j := 0; j < slLen; j++ {
			scopeLog := logs.ScopeLogs().At(j)
			scopeURL := scopeLog.SchemaUrl()
			scopeLogScope := scopeLog.Scope()
			scopeName := scopeLogScope.Name()
			scopeVersion := scopeLogScope.Version()
			scopeLogRecords := scopeLog.LogRecords()
			scopeAttributesBuffer.Reset()
			chjson.AttributesToJSON(scopeAttributesBuffer, scopeLogScope.Attributes())

			slrLen := scopeLogRecords.Len()
			for k := 0; k < slrLen; k++ {
				r := scopeLogRecords.At(k)
				logAttributesBuffer.Reset()
				chjson.AttributesToJSON(logAttributesBuffer, r.Attributes())

				timestamp := r.Timestamp()
				if timestamp == 0 {
					timestamp = r.ObservedTimestamp()
				}

				traceHexBuffer = chjson.AppendTraceIDToHex(traceHexBuffer[:0], r.TraceID())
				spanHexBuffer = chjson.AppendSpanIDToHex(spanHexBuffer[:0], r.SpanID())
				appendErr := batch.Append(
					timestamp.AsTime(),
					traceHexBuffer,
					spanHexBuffer,
					uint8(r.Flags()),
					r.SeverityText(),
					uint8(r.SeverityNumber()),
					serviceName,
					r.Body().Str(),
					resURL,
					resourceAttributesBuffer.Bytes(),
					scopeURL,
					scopeName,
					scopeVersion,
					scopeAttributesBuffer.Bytes(),
					logAttributesBuffer.Bytes(),
				)
				if appendErr != nil {
					return fmt.Errorf("failed to append log row: %w", appendErr)
				}

				logCount++
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("logs json insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	e.logger.Debug("insert json logs",
		zap.Int("records", logCount),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func renderInsertLogsJSONSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.LogsJSONInsert, cfg.database(), cfg.LogsTableName)
}

func renderCreateLogsJSONTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "TimestampTime")
	return fmt.Sprintf(sqltemplates.LogsJSONCreateTable,
		cfg.database(), cfg.LogsTableName, cfg.clusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func createLogsJSONTable(ctx context.Context, cfg *Config, db driver.Conn) error {
	if err := db.Exec(ctx, renderCreateLogsJSONTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create logs json table sql: %w", err)
	}

	return nil
}
