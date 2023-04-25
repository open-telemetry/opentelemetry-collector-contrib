// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type logsReceiver struct {
	config           *Config
	settings         receiver.CreateSettings
	createConnection dbProviderFunc
	createClient     clientProviderFunc
	queryReceivers   []*logsQueryReceiver
	nextConsumer     consumer.Logs
}

func newLogsReceiver(
	config *Config,
	settings receiver.CreateSettings,
	sqlOpenerFunc sqlOpenerFunc,
	createClient clientProviderFunc,
	nextConsumer consumer.Logs,
) (*logsReceiver, error) {
	receiver := &logsReceiver{
		config:   config,
		settings: settings,
		createConnection: func() (*sql.DB, error) {
			return sqlOpenerFunc(config.Driver, config.DataSource)
		},
		createClient: createClient,
		nextConsumer: nextConsumer,
	}

	receiver.createQueryReceivers()

	return receiver, nil
}

func (receiver *logsReceiver) createQueryReceivers() {
	for i, query := range receiver.config.Queries {
		if len(query.Logs) == 0 {
			continue
		}
		id := component.NewIDWithName("sqlqueryreceiver", fmt.Sprintf("query-%d: %s", i, query.SQL))
		queryReceiver := newLogsQueryReceiver(
			id,
			query,
			receiver.createConnection,
			receiver.createClient,
			receiver.settings.Logger,
		)
		receiver.queryReceivers = append(receiver.queryReceivers, queryReceiver)
	}
}

func (receiver *logsReceiver) Start(ctx context.Context, host component.Host) error {
	for _, queryReceiver := range receiver.queryReceivers {
		err := queryReceiver.start()
		if err != nil {
			return err
		}
	}
	for _, queryReceiver := range receiver.queryReceivers {
		logs, err := queryReceiver.scrape(ctx)
		if err != nil {
			return err
		}
		receiver.nextConsumer.ConsumeLogs(ctx, logs)
	}
	return nil
}

func (receiver *logsReceiver) Shutdown(ctx context.Context) error {
	for _, queryReceiver := range receiver.queryReceivers {
		queryReceiver.shutdown(ctx)
	}
	return nil
}

type logsQueryReceiver struct {
	id           component.ID
	query        Query
	createDb     dbProviderFunc
	createClient clientProviderFunc
	logger       *zap.Logger
	db           *sql.DB
	client       dbClient
}

func newLogsQueryReceiver(
	id component.ID,
	query Query,
	dbProviderFunc dbProviderFunc,
	clientProviderFunc clientProviderFunc,
	logger *zap.Logger,
) *logsQueryReceiver {
	queryReceiver := &logsQueryReceiver{
		id:           id,
		query:        query,
		createDb:     dbProviderFunc,
		createClient: clientProviderFunc,
		logger:       logger,
	}
	return queryReceiver
}

func (queryReceiver *logsQueryReceiver) start() error {
	var err error
	queryReceiver.db, err = queryReceiver.createDb()
	if err != nil {
		return fmt.Errorf("failed to open db connection: %w", err)
	}
	queryReceiver.client = queryReceiver.createClient(dbWrapper{queryReceiver.db}, queryReceiver.query.SQL, queryReceiver.logger)

	return nil
}

func (queryReceiver *logsQueryReceiver) scrape(ctx context.Context) (plog.Logs, error) {
	logs := plog.NewLogs()

	rows, err := queryReceiver.client.queryRows(ctx)
	if err != nil {
		return logs, fmt.Errorf("error getting rows: %w", err)
	}

	var errs error
	scopeLogs := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for _, logsConfig := range queryReceiver.query.Logs {
		for i, row := range rows {
			if err = rowToLog(row, logsConfig, scopeLogs.AppendEmpty()); err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = multierr.Append(errs, err)
			}
		}
	}
	return logs, nil
}

func rowToLog(row stringMap, config LogsCfg, logRecord plog.LogRecord) error {
	logRecord.Body().SetStr(row[config.BodyColumn])
	return nil
}

func (queryReceiver *logsQueryReceiver) shutdown(ctx context.Context) error {
	return nil
}
