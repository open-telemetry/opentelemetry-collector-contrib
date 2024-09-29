package postgresexporter

import (
	"database/sql"

	"go.uber.org/zap"
)

type metricsExporter struct {
	client *sql.DB

	logger *zap.Logger
}
