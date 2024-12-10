package postgresexporter

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsExporter struct {
	client    *sql.DB
	insertSQL string
	logger    *zap.Logger
	cfg       *Config
}

func newLogsExporter(logger *zap.Logger, cfg *Config) (*logsExporter, error) {
	client, err := newPostgresClient(cfg)
	if err != nil {
		return nil, err
	}

	return &logsExporter{
		client:    client,
		insertSQL: "",
		logger:    logger,
		cfg:       cfg,
	}, nil
}

func (e *logsExporter) start(ctx context.Context, _ component.Host) error {
	context.TODO()
	return nil
}

func (e *logsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	context.TODO()
	return nil
}

func newPostgresClient(cfg *Config) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database)
	db, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		return nil, err
	}
	return db, nil
}
