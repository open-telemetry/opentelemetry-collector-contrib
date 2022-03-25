// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go" // For register database driver.
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type clickhouseExporter struct {
	client        *sql.DB
	insertLogsSQL string

	logger *zap.Logger
	cfg    *Config
}

func newExporter(logger *zap.Logger, cfg *Config) (*clickhouseExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newClickhouseClient(cfg)
	if err != nil {
		return nil, err
	}

	insertLogsSQL := renderInsertLogsSQL(cfg)

	return &clickhouseExporter{
		client:        client,
		insertLogsSQL: insertLogsSQL,
		logger:        logger,
		cfg:           cfg,
	}, nil
}

// Shutdown will shutdown the exporter.
func (e *clickhouseExporter) Shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *clickhouseExporter) pushLogsData(ctx context.Context, ld pdata.Logs) error {
	start := time.Now()
	err := doWithTx(ctx, e.client, func(tx *sql.Tx) error {
		statement, err := tx.PrepareContext(ctx, e.insertLogsSQL)
		if err != nil {
			return fmt.Errorf("PrepareContext:%w", err)
		}
		defer func() {
			_ = statement.Close()
		}()
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			logs := ld.ResourceLogs().At(i)
			res := logs.Resource()
			resourceKeys, resourceValues := attributesToSlice(res.Attributes())
			for j := 0; j < logs.InstrumentationLibraryLogs().Len(); j++ {
				rs := logs.InstrumentationLibraryLogs().At(j).LogRecords()
				for k := 0; k < rs.Len(); k++ {
					r := rs.At(k)
					attrKeys, attrValues := attributesToSlice(r.Attributes())
					_, err = statement.ExecContext(ctx,
						r.Timestamp().AsTime(),
						r.TraceID().HexString(),
						r.SpanID().HexString(),
						r.Flags(),
						r.SeverityText(),
						r.SeverityNumber(),
						r.Body().AsString(),
						resourceKeys,
						resourceValues,
						attrKeys,
						attrValues,
					)
					if err != nil {
						return fmt.Errorf("ExecContext:%w", err)
					}
				}
			}
		}
		return nil
	})
	duration := time.Since(start)
	e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()),
		zap.String("cost", duration.String()))
	return err
}

func attributesToSlice(attributes pdata.Map) ([]string, []string) {
	keys := make([]string, 0, attributes.Len())
	values := make([]string, 0, attributes.Len())
	attributes.Range(func(k string, v pdata.Value) bool {
		keys = append(keys, formatKey(k))
		values = append(values, v.AsString())
		return true
	})
	return keys, values
}

func formatKey(k string) string {
	return strings.ReplaceAll(k, ".", "_")
}

const (
	// language=ClickHouse SQL
	createLogsTableSQL = `
CREATE TABLE IF NOT EXISTS %s (
     Timestamp DateTime CODEC(Delta, ZSTD(1)),
     TraceId String CODEC(ZSTD(1)),
     SpanId String CODEC(ZSTD(1)),
     TraceFlags UInt32,
     SeverityText LowCardinality(String) CODEC(ZSTD(1)),
     SeverityNumber Int32,
     Body String CODEC(ZSTD(1)),
     ResourceAttributes Nested
     (
         Key LowCardinality(String),
         Value String
     ) CODEC(ZSTD(1)),
     LogAttributes Nested
     (
         Key LowCardinality(String),
         Value String
     ) CODEC(ZSTD(1)),
     INDEX idx_attr_keys ResourceAttributes.Key TYPE bloom_filter(0.01) GRANULARITY 64,
     INDEX idx_res_keys LogAttributes.Key TYPE bloom_filter(0.01) GRANULARITY 64
) ENGINE MergeTree()
%s
PARTITION BY toDate(Timestamp)
ORDER BY (toUnixTimestamp(Timestamp));
`
	// language=ClickHouse SQL
	insertLogsSQLTemplate = `INSERT INTO %s (
                        Timestamp,
                        TraceId,
                        SpanId,
                        TraceFlags,
                        SeverityText,
                        SeverityNumber,
                        Body,
                        ResourceAttributes.Key,
                        ResourceAttributes.Value,
                        LogAttributes.Key, 
                        LogAttributes.Value
                        ) VALUES (
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?
                                  )`
)

var driverName = "clickhouse" // for testing

// newClickhouseClient create a clickhouse client.
func newClickhouseClient(cfg *Config) (*sql.DB, error) {
	// use empty database to create database
	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sql.Open:%w", err)
	}
	// create table
	query := fmt.Sprintf(createLogsTableSQL, cfg.LogsTableName, "")
	if cfg.TTLDays > 0 {
		query = fmt.Sprintf(createLogsTableSQL,
			cfg.LogsTableName,
			fmt.Sprintf(`TTL Timestamp + INTERVAL %d DAY`, cfg.TTLDays))
	}
	if _, err := db.Exec(query); err != nil {
		return nil, fmt.Errorf("exec create table sql: %w", err)
	}
	return db, nil
}

func renderInsertLogsSQL(cfg *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplate, cfg.LogsTableName)
}

func doWithTx(_ context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("db.Begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
