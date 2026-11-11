// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"context"
	"encoding/json"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type logsExporter struct {
	client *gocql.Session
	exec   queryExecutor
	logger *zap.Logger
	cfg    *Config
}

func newLogsExporter(logger *zap.Logger, cfg *Config) *logsExporter {
	return &logsExporter{logger: logger, cfg: cfg}
}

func (e *logsExporter) Start(ctx context.Context, _ component.Host) error {
	if err := initializeLogKernel(ctx, e.cfg); err != nil {
		return err
	}
	session, err := newDataSession(e.cfg)
	if err != nil {
		return err
	}
	e.client = session
	e.exec = gocqlExecutor{session: session}
	return nil
}

func (e *logsExporter) Shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.Close()
	}
	return nil
}

func (e *logsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	start := time.Now()
	rows, err := logRows(ld)
	if err != nil {
		return err
	}
	if err := writeRows(ctx, e.exec, parseInsertLogSQL(e.cfg), rows); err != nil {
		return err
	}
	e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()), zap.String("cost", time.Since(start).String()))
	return nil
}

func logRows(ld plog.Logs) ([]insertRow, error) {
	rows := make([]insertRow, 0, ld.LogRecordCount())
	index := 0
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLogs := ld.ResourceLogs().At(i)
		resourceAttrs := attributesToMap(resourceLogs.Resource().Attributes().AsRaw())
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			records := resourceLogs.ScopeLogs().At(j).LogRecords()
			for k := 0; k < records.Len(); k++ {
				record := records.At(k)
				body, err := json.Marshal(record.Body().AsRaw())
				if err != nil {
					return nil, err
				}
				traceID := traceutil.TraceIDToHexOrEmptyString(record.TraceID())
				spanID := traceutil.SpanIDToHexOrEmptyString(record.SpanID())
				ts := record.Timestamp().AsTime()
				logAttrs := attributesToMap(record.Attributes().AsRaw())
				partition := logPartitionKey(traceID, ts)
				flags := uint32(record.Flags())
				severity := int32(record.SeverityNumber())
				rows = append(rows, insertRow{
					partition: partition,
					args: []any{
						logRecordID(index, partition, traceID, spanID, ts, flags, record.SeverityText(), severity, string(body), resourceAttrs, logAttrs),
						ts,
						partition,
						traceID,
						spanID,
						int32(flags),
						record.SeverityText(),
						severity,
						string(body),
						resourceAttrs,
						logAttrs,
					},
				})
				index++
			}
		}
	}
	return rows, nil
}
