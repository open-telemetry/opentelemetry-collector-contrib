package postgresexporter

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
)

type tracesExporter struct {
	client    *sql.DB
	insertSQL string
	logger    *zap.Logger
	cfg       *Config
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (*tracesExporter, error) {
	client, err := newPostgresClient(cfg)
	if err != nil {
		return nil, err
	}

	return &tracesExporter{
		client:    client,
		insertSQL: "", 
		logger:    logger,
		cfg:       cfg,
	}, nil
}

func (e *tracesExporter) start(ctx context.Context, _ component.Host) error {
	return nil 
}

func (e *tracesExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}


func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
    return nil
}

