// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type logsExporter struct {
	client *gocql.Session
	logger *zap.Logger
	cfg    *Config
}

func newLogsExporter(logger *zap.Logger, cfg *Config) (*logsExporter, error) {
	cluster := gocql.NewCluster(cfg.DSN)
	session, err := cluster.CreateSession()
	cluster.Keyspace = cfg.Keyspace
	cluster.Consistency = gocql.Quorum

	if err != nil {
		return nil, err
	}

	return &logsExporter{logger: logger, client: session, cfg: cfg}, nil
}

func initializeLogKernel(cfg *Config) error {
	ctx := context.Background()
	cluster := gocql.NewCluster(cfg.DSN)
	cluster.Consistency = gocql.Quorum
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}

	defer session.Close()

	createDatabaseError := session.Query(parseCreateDatabaseSQL(cfg)).WithContext(ctx).Exec()
	if createDatabaseError != nil {
		return createDatabaseError
	}
	createLogTableError := session.Query(parseCreateLogTableSQL(cfg)).WithContext(ctx).Exec()
	if createLogTableError != nil {
		return createLogTableError
	}

	return nil
}

func (e *logsExporter) Start(ctx context.Context, host component.Host) error {
	initializeErr := initializeLogKernel(e.cfg)
	return initializeErr
}

func (e *logsExporter) Shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.Close()
	}

	return nil
}

func parseCreateLogTableSQL(cfg *Config) string {
	return fmt.Sprintf(createLogTableSQL, cfg.Keyspace, cfg.LogsTable, cfg.Compression.Algorithm)
}

func (e *logsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	start := time.Now()

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		logs := ld.ResourceLogs().At(i)
		res := logs.Resource()
		resAttr := attributesToMap(res.Attributes().AsRaw())

		for j := 0; j < logs.ScopeLogs().Len(); j++ {
			rs := logs.ScopeLogs().At(j).LogRecords()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				logAttr := attributesToMap(r.Attributes().AsRaw())
				bodyByte, _ := json.Marshal(r.Body().AsRaw())

				insertLogError := e.client.Query(fmt.Sprintf(insertLogTableSQL, e.cfg.Keyspace, e.cfg.LogsTable),
					r.Timestamp().AsTime(),
					traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					uint32(r.Flags()),
					r.SeverityText(),
					int32(r.SeverityNumber()),
					string(bodyByte),
					resAttr,
					logAttr,
				).WithContext(ctx).Exec()

				if insertLogError != nil {
					e.logger.Error("insert log error", zap.Error(insertLogError))
				}
			}
		}
	}

	duration := time.Since(start)
	e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()),
		zap.String("cost", duration.String()))
	return nil
}
