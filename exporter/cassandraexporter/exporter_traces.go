// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type tracesExporter struct {
	client *gocql.Session
	logger *zap.Logger
	cfg    *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (*tracesExporter, error) {
	cluster := gocql.NewCluster(cfg.DSN)
	session, err := cluster.CreateSession()
	cluster.Keyspace = cfg.Keyspace
	cluster.Consistency = gocql.Quorum

	if err != nil {
		return nil, err
	}

	return &tracesExporter{logger: logger, client: session, cfg: cfg}, nil
}

func initializeTraceKernel(cfg *Config) error {
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
	createLinksTypeError := session.Query(parseCreateLinksTypeSQL(cfg)).WithContext(ctx).Exec()
	if createLinksTypeError != nil {
		return createLinksTypeError
	}
	createEventsTypeError := session.Query(parseCreateEventsTypeSQL(cfg)).WithContext(ctx).Exec()
	if createEventsTypeError != nil {
		return createEventsTypeError
	}
	createSpanTableError := session.Query(parseCreateSpanTableSQL(cfg)).WithContext(ctx).Exec()
	if createSpanTableError != nil {
		return createSpanTableError
	}

	return nil
}

func parseCreateSpanTableSQL(cfg *Config) string {
	return fmt.Sprintf(createSpanTableSQL, cfg.Keyspace, cfg.TraceTable, cfg.Compression.Algorithm)
}

func parseCreateEventsTypeSQL(cfg *Config) string {
	return fmt.Sprintf(createEventTypeSQL, cfg.Keyspace)
}

func parseCreateLinksTypeSQL(cfg *Config) string {
	return fmt.Sprintf(createLinksTypeSQL, cfg.Keyspace)
}

func parseCreateDatabaseSQL(cfg *Config) string {
	return fmt.Sprintf(createDatabaseSQL, cfg.Keyspace, cfg.Replication.Class, cfg.Replication.ReplicationFactor)
}

func (e *tracesExporter) Start(ctx context.Context, host component.Host) error {
	initializeErr := initializeTraceKernel(e.cfg)
	return initializeErr
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
		resAttr := attributesToMap(res.Attributes().AsRaw())

		for j := 0; j < spans.ScopeSpans().Len(); j++ {
			rs := spans.ScopeSpans().At(j).Spans()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				spanAttr := attributesToMap(r.Attributes().AsRaw())
				status := r.Status()

				insertSpanError := e.client.Query(fmt.Sprintf(insertSpanSQL, e.cfg.Keyspace, e.cfg.TraceTable), r.StartTimestamp().AsTime(),
					traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					traceutil.SpanIDToHexOrEmptyString(r.ParentSpanID()),
					r.TraceState().AsRaw(),
					r.Name(),
					traceutil.SpanKindStr(r.Kind()),
					resAttr,
					spanAttr,
					r.EndTimestamp().AsTime().Sub(r.StartTimestamp().AsTime()).Nanoseconds(),
					traceutil.StatusCodeStr(status.Code()),
					status.Message(),
				).WithContext(ctx).Exec()

				if insertSpanError != nil {
					e.logger.Error("insert span error", zap.Error(insertSpanError))
				}
			}
		}
	}

	duration := time.Since(start)
	e.logger.Debug("insert traces", zap.Int("records", td.SpanCount()),
		zap.String("cost", duration.String()))
	return nil
}
