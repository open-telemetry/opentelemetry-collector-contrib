package postgresexporter

import (
	"context"
	"database/sql"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsExporter struct {
	client    *sql.DB
	insertSQL string
	logger    *zap.Logger
	cfg       *Config
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) (*metricsExporter, error) {
	client, err := newPostgresClient(cfg)
	if err != nil {
		return nil, err
	}

	return &metricsExporter{
		client:    client,
		insertSQL: "",
		logger:    logger,
		cfg:       cfg,
	}, nil
}

func (e *metricsExporter) start(ctx context.Context, _ component.Host) error {
	context.TODO()
	return nil
}

func (e *metricsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *metricsExporter) pushMetricData(ctx context.Context, td pmetric.Metrics) error {
	context.TODO()
	return nil
}
