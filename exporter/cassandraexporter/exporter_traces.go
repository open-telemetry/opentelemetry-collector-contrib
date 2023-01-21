package cassandraexporter

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gocql/gocql"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"time"
)

type tracesExporter struct {
	client *sql.DB
	logger *zap.Logger
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (*tracesExporter, error) {
	ctx := context.Background()
	cluster := gocql.NewCluster("127.0.0.1")
	session, _ := cluster.CreateSession()
	session.Query(`INSERT INTO tweet (timeline, id, text) VALUES (?, ?, ?)`,
		"me", gocql.TimeUUID(), "hello world").WithContext(ctx).Exec()

	return &tracesExporter{logger: logger}, nil
}

func (e *tracesExporter) Shutdown(_ context.Context) error {
	return nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	start := time.Now()

	fmt.Println("here")
	fmt.Println(td.SpanCount())

	duration := time.Since(start)
	e.logger.Info("insert traces", zap.Int("records", td.SpanCount()),
		zap.String("cost", duration.String()))
	return nil
}
