package postgresexporter

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type postgresexporter struct{}

func NewPostgresExporter() *postgresexporter {
	return &postgresexporter{}
}

func (s *postgresexporter) pushLogs(_ context.Context, ld plog.Logs) error {
	return nil
}

func (s *postgresexporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (s *postgresexporter) pushTraces(_ context.Context, td ptrace.Traces) error {
	return nil
}
