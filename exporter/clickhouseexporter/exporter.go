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

// Shutdown will shutdown the exporter.
func (e *clickhouseExporter) Shutdown(ctx context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *clickhouseExporter) pushLogsData(ctx context.Context, ld pdata.Logs) error {
	// TODO implement write data to clickhouse
	return nil
}

// newClickhouseClient create a clickhouse client.
func newClickhouseClient(cfg *Config) (*sql.DB, error) {
	// TODO implement create clickhouse client
	return nil, nil
}
