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
	"net/url"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type clickhouseExporter struct {
	client *sql.DB

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

	return &clickhouseExporter{
		client: client,
		logger: logger,
		cfg:    cfg,
	}, nil
}

func (e *clickhouseExporter) Shutdown(ctx context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *clickhouseExporter) pushLogsData(ctx context.Context, ld pdata.Logs) error {
	return writeLogs(ctx, e.client, ld)
}

// newClickhouseClient create a clickhouse client.
func newClickhouseClient(cfg *Config) (*sql.DB, error) {
	// TODO implement
	params := url.Values{}
	params.Set("database", cfg.Database)
	params.Set("password", cfg.Password)
	params.Set("password", cfg.Password)
	db, err := sql.Open("clickhouse", fmt.Sprintf("%s?%s", cfg.Address, params.Encode()))
	if err != nil {
		return nil, err
	}
	return db, nil
}

const (
	// language=ClickHouse SQL
	createLogsTableSQL = `
CREATE TABLE IF NOT EXISTS logs (
     timestamp DateTime CODEC(Delta, ZSTD(1)),
     TraceId String CODEC(ZSTD(1)),
     SpanId String CODEC(ZSTD(1)),
     TraceFlags Int64,
     SeverityText LowCardinality(String) CODEC(ZSTD(1)),
     SeverityNumber Int64,
     Name LowCardinality(String) CODEC(ZSTD(1)),
     Body String CODEC(ZSTD(1)),
     Attributes Nested
     (
         key LowCardinality(String),
         value String
     ) CODEC(ZSTD(1)),
     Resource Nested
     (
         key LowCardinality(String),
         value String
     ) CODEC(ZSTD(1)),
     INDEX idx_attr_keys Attributes.key TYPE bloom_filter(0.01) GRANULARITY 64,
     INDEX idx_res_keys Resource.key TYPE bloom_filter(0.01) GRANULARITY 64
) ENGINE MergeTree()
TTL timestamp + INTERVAL 3 DAY
PARTITION BY toDate(timestamp)
ORDER BY (Name, -toUnixTimestamp(timestamp))
SETTINGS index_granularity=1024
`
	// TODO implement
	// language=ClickHouse SQL
	insertLogsSQL = ``
)

// writeLogs write logs to clickhouse.
func writeLogs(ctx context.Context, db *sql.DB, ld pdata.Logs) error {
	var errs []error
	txFn := func(tx *sql.Tx) error {
		query := insertLogsSQL
		statement, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return err
		}
		defer func() {
			_ = statement.Close()
		}()
		rls := ld.ResourceLogs()
		for i := 0; i < rls.Len(); i++ {
			rl := rls.At(i)
			resource := rl.Resource()
			ills := rl.InstrumentationLibraryLogs()
			for j := 0; j < ills.Len(); j++ {
				logs := ills.At(i).Logs()
				rKeys, rValues := attrsKeyValue(resource.Attributes())
				for k := 0; k < logs.Len(); k++ {
					record := logs.At(k)
					keys, values := attrsKeyValue(record.Attributes())
					_, err = statement.ExecContext(ctx,
						record.Timestamp().AsTime(),
						record.TraceID().HexString(),
						record.SpanID().HexString(),
						int64(record.Flags()),
						record.SeverityText(),
						int64(record.SeverityNumber()),
						record.Name(),
						record.Body().AsString(),
						keys, values,
						rKeys, rValues,
					)
					if err != nil {
						if cerr := ctx.Err(); cerr != nil {
							return cerr
						}
						errs = append(errs, err)
					}
				}
			}
		}
		return nil
	}
	return doWithTx(ctx, db, txFn)
}

func doWithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func attrsKeyValue(attrs pdata.AttributeMap) ([]string, []string) {
	keys := make([]string, 0, attrs.Len())
	values := make([]string, 0, attrs.Len())
	attrs.Range(func(k string, v pdata.AttributeValue) bool {
		keys = append(keys, k)
		values = append(values, v.AsString())
		return true
	})
	return keys, values
}
