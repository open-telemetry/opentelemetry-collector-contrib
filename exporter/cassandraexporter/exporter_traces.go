package cassandraexporter

import (
	"context"
	"fmt"
	"github.com/gocql/gocql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"time"
)

type tracesExporter struct {
	client *gocql.Session
	logger *zap.Logger
	cfg    *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (*tracesExporter, error) {
	initializeKernel(cfg)
	cluster := gocql.NewCluster(cfg.DSN)
	session, err := cluster.CreateSession()
	cluster.Keyspace = cfg.Keyspace
	cluster.Consistency = gocql.Quorum

	if err != nil {
		return nil, err
	}

	return &tracesExporter{logger: logger, client: session, cfg: cfg}, nil
}

func initializeKernel(cfg *Config) error {
	ctx := context.Background()
	cluster := gocql.NewCluster(cfg.DSN)
	cluster.Consistency = gocql.Quorum
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}

	session.Query(parseCreateDatabaseSql(cfg)).WithContext(ctx).Exec()
	session.Query(parseCreateLinksTypeSql(cfg)).WithContext(ctx).Exec()
	session.Query(parseCreateEventsTypeSql(cfg)).WithContext(ctx).Exec()
	session.Query(parseCreateSpanTableSql(cfg)).WithContext(ctx).Exec()

	defer session.Close()

	return nil
}

func parseCreateSpanTableSql(cfg *Config) string {
	return fmt.Sprintf(createSpanTableSQL, cfg.Keyspace, cfg.TraceTable)
}

func parseCreateEventsTypeSql(cfg *Config) string {
	return fmt.Sprintf(createEventTypeSql, cfg.Keyspace)
}

func parseCreateLinksTypeSql(cfg *Config) string {
	return fmt.Sprintf(createLinksTypeSql, cfg.Keyspace)
}

func parseCreateDatabaseSql(cfg *Config) string {
	return fmt.Sprintf(createDatabaseSQL, cfg.Keyspace)
}

func (e *tracesExporter) Shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.Close()
	}

	return nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	start := time.Now()

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		spans := td.ResourceSpans().At(i)
		res := spans.Resource()
		resAttr := attributesToMap(res.Attributes())
		var serviceName string
		if v, ok := res.Attributes().Get(conventions.AttributeServiceName); ok {
			serviceName = v.Str()
		}
		for j := 0; j < spans.ScopeSpans().Len(); j++ {
			rs := spans.ScopeSpans().At(j).Spans()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				spanAttr := attributesToMap(r.Attributes())
				status := r.Status()

				e.client.Query(fmt.Sprintf(`INSERT INTO %s.%s (timestamp, traceid, spanid, parentspanid, tracestate, spanname, spankind, servicename, resourceattributes, spanattributes, duration, statuscode, statusmessage) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, e.cfg.Keyspace, e.cfg.TraceTable), r.StartTimestamp().AsTime(),
					traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					traceutil.SpanIDToHexOrEmptyString(r.ParentSpanID()),
					r.TraceState().AsRaw(),
					r.Name(),
					traceutil.SpanKindStr(r.Kind()),
					serviceName,
					resAttr,
					spanAttr,
					r.EndTimestamp().AsTime().Sub(r.StartTimestamp().AsTime()).Nanoseconds(),
					traceutil.StatusCodeStr(status.Code()),
					status.Message(),
				).WithContext(ctx).Exec()
			}
		}
	}

	duration := time.Since(start)
	e.logger.Info("insert traces", zap.Int("records", td.SpanCount()),
		zap.String("cost", duration.String()))
	return nil
}

func attributesToMap(attributes pcommon.Map) map[string]string {
	m := make(map[string]string, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		m[k] = v.AsString()
		return true
	})
	return m
}

const (
	// language=SQL
	createDatabaseSQL = `CREATE KEYSPACE %s with replication = { 'class' : 'SimpleStrategy', 'replication_factor' : 1 };`
	// language=SQL
	createEventTypeSql = `CREATE TYPE IF NOT EXISTS %s.Events (Timestamp Date, Name text, Attributes map<text, text>);`
	// language=SQL
	createLinksTypeSql = `CREATE TYPE IF NOT EXISTS %s.Links (TraceId text, SpanId text, TraceState text, Attributes map<text, text>);`
	// language=SQL
	createSpanTableSQL = `CREATE TABLE IF NOT EXISTS %s.%s (TimeStamp DATE,TraceId text, SpanId text, ParentSpanId text, TraceState text, SpanName text, SpanKind text, ServiceName text, ResourceAttributes map<text, text>, SpanAttributes map<text, text>, Duration int,StatusCode text,StatusMessage text, Events frozen<Events>, Links frozen<Links>, PRIMARY KEY (TraceId));`
)
