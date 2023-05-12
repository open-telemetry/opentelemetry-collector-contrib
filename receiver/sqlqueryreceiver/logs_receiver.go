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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal/observability"
)

type logsReceiver struct {
	config           *Config
	settings         receiver.CreateSettings
	createConnection dbProviderFunc
	createClient     clientProviderFunc
	queryReceivers   []*logsQueryReceiver
	nextConsumer     consumer.Logs

	isStarted                bool
	collectionIntervalTicker *time.Ticker
	shutdownRequested        chan struct{}

	id component.ID
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
		createClient:      createClient,
		nextConsumer:      nextConsumer,
		shutdownRequested: make(chan struct{}),
		id:                settings.ID,
	}

	receiver.createQueryReceivers()

	return receiver, nil
}

func (receiver *logsReceiver) createQueryReceivers() {
	for i, query := range receiver.config.Queries {
		if len(query.Logs) == 0 {
			continue
		}
		id := fmt.Sprintf("query-%d: %s", i, query.SQL)
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
	if receiver.isStarted {
		receiver.settings.Logger.Debug("requested start, but already started, ignoring.")
		return nil
	}
	receiver.settings.Logger.Debug("starting...")
	receiver.isStarted = true

	for _, queryReceiver := range receiver.queryReceivers {
		err := queryReceiver.start()
		if err != nil {
			return err
		}
	}
	receiver.startCollecting()
	receiver.settings.Logger.Debug("started.")
	return nil
}

func (receiver *logsReceiver) startCollecting() {
	receiver.collectionIntervalTicker = time.NewTicker(receiver.config.CollectionInterval)

	go func() {
		for {
			select {
			case <-receiver.collectionIntervalTicker.C:
				receiver.collect()
			case <-receiver.shutdownRequested:
				return
			}
		}
	}()
}

func (receiver *logsReceiver) collect() {
	logsChannel := make(chan plog.Logs)
	for _, queryReceiver := range receiver.queryReceivers {
		go func(queryReceiver *logsQueryReceiver) {
			logs, err := queryReceiver.collect(context.Background())
			if err != nil {
				receiver.settings.Logger.Error("Error collecting logs", zap.Error(err), zap.String("query", queryReceiver.ID()))
			}

			if err := observability.RecordAcceptedLogs(int64(logs.LogRecordCount()), receiver.id.String(), queryReceiver.id); err != nil {
				receiver.settings.Logger.Debug("error recording metric for number of collected logs", zap.Error(err))
			}

			logsChannel <- logs
		}(queryReceiver)
	}

	allLogs := plog.NewLogs()
	for range receiver.queryReceivers {
		logs := <-logsChannel
		logs.ResourceLogs().MoveAndAppendTo(allLogs.ResourceLogs())
	}
	if allLogs.LogRecordCount() > 0 {
		receiver.nextConsumer.ConsumeLogs(context.Background(), allLogs)
	}
}

func (receiver *logsReceiver) Shutdown(ctx context.Context) error {
	if !receiver.isStarted {
		receiver.settings.Logger.Debug("Requested shutdown, but not started, ignoring.")
		return nil
	}

	receiver.settings.Logger.Debug("stopping...")
	receiver.stopCollecting()
	for _, queryReceiver := range receiver.queryReceivers {
		queryReceiver.shutdown(ctx)
	}

	receiver.isStarted = false
	receiver.settings.Logger.Debug("stopped.")

	return nil
}

func (receiver *logsReceiver) stopCollecting() {
	receiver.collectionIntervalTicker.Stop()
	close(receiver.shutdownRequested)
}

type logsQueryReceiver struct {
	id           string
	query        Query
	createDb     dbProviderFunc
	createClient clientProviderFunc
	logger       *zap.Logger

	db            *sql.DB
	client        dbClient
	trackingValue string
}

func newLogsQueryReceiver(
	id string,
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
	queryReceiver.trackingValue = queryReceiver.query.TrackingStartValue
	return queryReceiver
}

func (queryReceiver *logsQueryReceiver) ID() string {
	return queryReceiver.id
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

func (queryReceiver *logsQueryReceiver) collect(ctx context.Context) (plog.Logs, error) {
	logs := plog.NewLogs()

	var rows []stringMap
	var err error
	if queryReceiver.query.TrackingColumn != "" {
		rows, err = queryReceiver.client.queryRows(ctx, queryReceiver.trackingValue)
	} else {
		rows, err = queryReceiver.client.queryRows(ctx)
	}
	if err != nil {
		return logs, fmt.Errorf("error getting rows: %w", err)
	}

	var errs error
	scopeLogs := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for logsConfigIndex, logsConfig := range queryReceiver.query.Logs {
		for i, row := range rows {
			if err = rowToLog(row, logsConfig, scopeLogs.AppendEmpty()); err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = multierr.Append(errs, err)
			}
			if logsConfigIndex == 0 {
				queryReceiver.storeTrackingValue(row)
			}
		}
	}
	return logs, nil
}

func (queryReceiver *logsQueryReceiver) storeTrackingValue(row stringMap) {
	if queryReceiver.query.TrackingColumn == "" {
		return
	}
	currentTrackingColumnValueString := row[queryReceiver.query.TrackingColumn]
	queryReceiver.trackingValue = currentTrackingColumnValueString
}

func rowToLog(row stringMap, config LogsCfg, logRecord plog.LogRecord) error {
	logRecord.Body().SetStr(row[config.BodyColumn])
	return nil
}

func (queryReceiver *logsQueryReceiver) shutdown(ctx context.Context) error {
	return nil
}
