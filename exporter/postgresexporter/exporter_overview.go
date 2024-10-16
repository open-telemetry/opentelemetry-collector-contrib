package postgresexporter

import (
	"context"
	"database/sql"
	"log"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type postgresexporter struct {
	client *sql.DB
	logger *zap.Logger
	cfg    *Config
}

func NewPostgresExporter(cfg *Config) *postgresexporter {
	return &postgresexporter{
		cfg: cfg,
	}
}

func (s *postgresexporter) pushLogs(_ context.Context, ld plog.Logs) error {
	log.Println(ld)
	return nil
}

func (s *postgresexporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (s *postgresexporter) pushTraces(_ context.Context, td ptrace.Traces) error {
	return nil
}
