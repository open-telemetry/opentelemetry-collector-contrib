package cassandraexporter

import (
	"context"
	"fmt"
	"github.com/gocql/gocql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"time"
)

type logsExporter struct {
	client *gocql.Session
	logger *zap.Logger
	cfg    *Config
}

func newLogsExporter(logger *zap.Logger, cfg *Config) (*logsExporter, error) {
	initializeErr := initializeLogKernel(cfg)
	if initializeErr != nil {
		return nil, initializeErr
	}

	cluster := gocql.NewCluster(cfg.DSN)
	session, err := cluster.CreateSession()
	cluster.Keyspace = cfg.Keyspace
	cluster.Consistency = gocql.Quorum

	if err != nil {
		return nil, err
	}

	return &logsExporter{logger: logger, client: session, cfg: cfg}, nil
}

func (e *logsExporter) Shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.Close()
	}

	return nil
}

func initializeLogKernel(cfg *Config) error {
	ctx := context.Background()
	cluster := gocql.NewCluster(cfg.DSN)
	cluster.Consistency = gocql.Quorum
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}

	session.Query(parseCreateDatabaseSql(cfg)).WithContext(ctx).Exec()
	session.Query(parseCreateLogTableSql(cfg)).WithContext(ctx).Exec()

	defer session.Close()

	return nil
}

func parseCreateLogTableSql(cfg *Config) string {
	return fmt.Sprintf(createLogTableSQL, cfg.Keyspace, cfg.LogsTable, cfg.Compression.Algorithm)
}

func (e *logsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	start := time.Now()

	var serviceName string
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		logs := ld.ResourceLogs().At(i)
		res := logs.Resource()
		resAttr := attributesToMap(res.Attributes())
		if v, ok := res.Attributes().Get(conventions.AttributeServiceName); ok {
			serviceName = v.Str()
		}
		for j := 0; j < logs.ScopeLogs().Len(); j++ {
			rs := logs.ScopeLogs().At(j).LogRecords()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				logAttr := attributesToMap(r.Attributes())

				fmt.Println("LOGS", r.SeverityText())

				e.client.Query(fmt.Sprintf(insertLogTableSQL, e.cfg.Keyspace, e.cfg.LogsTable),
					r.Timestamp().AsTime(),
					traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					uint32(r.Flags()),
					r.SeverityText(),
					int32(r.SeverityNumber()),
					serviceName,
					r.Body().AsString(),
					resAttr,
					logAttr,
				).WithContext(ctx).Exec()
			}
		}
	}

	duration := time.Since(start)
	e.logger.Info("insert logs", zap.Int("records", ld.LogRecordCount()),
		zap.String("cost", duration.String()))
	return nil
}
