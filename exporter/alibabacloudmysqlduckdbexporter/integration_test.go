// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package alibabacloudmysqlduckdbexporter

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const envMySQLDSN = "MYSQL_DSN"

var telemetryTimestamp = time.Date(2024, 12, 25, 12, 0, 29, 0, time.UTC)

func getMySQLDSN(t *testing.T) string {
	dsn := os.Getenv(envMySQLDSN)
	if dsn == "" {
		t.Skipf("Skipping integration test: %s not set", envMySQLDSN)
	}
	return dsn
}

func newTestConfig(dsn string) *Config {
	return &Config{
		TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		Endpoint:         dsn,
		Database:         "otel_test",
		LogsTableName:    "otel_logs",
		TracesTableName:  "otel_traces",
		MetricsTableName: "otel_metrics",
		CreateSchema:     true,
		TTL:              72 * time.Hour,
	}
}

// queryDB opens a direct connection for verification queries.
func queryDB(t *testing.T, dsn string) *sql.DB {
	db, err := sql.Open("mysql", dsn+"?parseTime=true")
	if err != nil {
		t.Fatalf("failed to open mysql for verification: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// truncateTable clears a table before test.
func truncateTable(t *testing.T, db *sql.DB, database, table string) {
	_, err := db.Exec("TRUNCATE TABLE `" + database + "`.`" + table + "`")
	if err != nil {
		t.Fatalf("failed to truncate table %s.%s: %v", database, table, err)
	}
}
