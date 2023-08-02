// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal/metadata"
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

	id            component.ID
	storageClient storage.Client
	obsrecv       *obsreport.Receiver
}

func newLogsReceiver(
	config *Config,
	settings receiver.CreateSettings,
	sqlOpenerFunc sqlOpenerFunc,
	createClient clientProviderFunc,
	nextConsumer consumer.Logs,
) (*logsReceiver, error) {

	obsr, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

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
		obsrecv:           obsr,
	}

	return receiver, nil
}

func (receiver *logsReceiver) Start(ctx context.Context, host component.Host) error {
	if receiver.isStarted {
		receiver.settings.Logger.Debug("requested start, but already started, ignoring.")
		return nil
	}
	receiver.settings.Logger.Debug("starting...")
	receiver.isStarted = true

	var err error
	receiver.storageClient, err = adapter.GetStorageClient(ctx, host, receiver.config.StorageID, receiver.settings.ID)
	if err != nil {
		return fmt.Errorf("error connecting to storage: %w", err)
	}

	err = receiver.createQueryReceivers()
	if err != nil {
		return err
	}

	for _, queryReceiver := range receiver.queryReceivers {
		err := queryReceiver.start(ctx)
		if err != nil {
			return err
		}
	}
	receiver.startCollecting()
	receiver.settings.Logger.Debug("started.")
	return nil
}

func (receiver *logsReceiver) createQueryReceivers() error {
	receiver.queryReceivers = nil
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
			receiver.storageClient,
		)
		receiver.queryReceivers = append(receiver.queryReceivers, queryReceiver)
	}
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
				receiver.settings.Logger.Error("error collecting logs", zap.Error(err), zap.String("query", queryReceiver.ID()))
			}
			logsChannel <- logs
		}(queryReceiver)
	}

	allLogs := plog.NewLogs()
	for range receiver.queryReceivers {
		logs := <-logsChannel
		logs.ResourceLogs().MoveAndAppendTo(allLogs.ResourceLogs())
	}

	logRecordCount := allLogs.LogRecordCount()
	if logRecordCount > 0 {
		ctx := receiver.obsrecv.StartLogsOp(context.Background())
		err := receiver.nextConsumer.ConsumeLogs(context.Background(), allLogs)
		receiver.obsrecv.EndLogsOp(ctx, metadata.Type, logRecordCount, err)
		if err != nil {
			receiver.settings.Logger.Error("failed to send logs: %w", zap.Error(err))
		}
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

	var errors error
	if receiver.storageClient != nil {
		errors = multierr.Append(errors, receiver.storageClient.Close(ctx))
	}

	receiver.isStarted = false
	receiver.settings.Logger.Debug("stopped.")

	return errors
}

func (receiver *logsReceiver) stopCollecting() {
	if receiver.collectionIntervalTicker != nil {
		receiver.collectionIntervalTicker.Stop()
	}
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
	// TODO: Extract persistence into its own component
	storageClient           storage.Client
	trackingValueStorageKey string
}

func newLogsQueryReceiver(
	id string,
	query Query,
	dbProviderFunc dbProviderFunc,
	clientProviderFunc clientProviderFunc,
	logger *zap.Logger,
	storageClient storage.Client,
) *logsQueryReceiver {
	queryReceiver := &logsQueryReceiver{
		id:            id,
		query:         query,
		createDb:      dbProviderFunc,
		createClient:  clientProviderFunc,
		logger:        logger,
		storageClient: storageClient,
	}
	queryReceiver.trackingValue = queryReceiver.query.TrackingStartValue
	queryReceiver.trackingValueStorageKey = fmt.Sprintf("%s.%s", queryReceiver.id, "trackingValue")
	return queryReceiver
}

func (queryReceiver *logsQueryReceiver) ID() string {
	return queryReceiver.id
}

func (queryReceiver *logsQueryReceiver) start(ctx context.Context) error {
	var err error
	queryReceiver.db, err = queryReceiver.createDb()
	if err != nil {
		return fmt.Errorf("failed to open db connection: %w", err)
	}
	queryReceiver.client = queryReceiver.createClient(dbWrapper{queryReceiver.db}, queryReceiver.query.SQL, queryReceiver.logger)

	queryReceiver.trackingValue = queryReceiver.retrieveTrackingValue(ctx)

	return nil
}

// retrieveTrackingValue retrieves the tracking value from storage, if storage is configured.
// Otherwise, it returns the tracking value configured in `tracking_start_value`.
func (queryReceiver *logsQueryReceiver) retrieveTrackingValue(ctx context.Context) string {
	trackingValueFromConfig := queryReceiver.query.TrackingStartValue
	if queryReceiver.storageClient == nil {
		return trackingValueFromConfig
	}

	storedTrackingValueBytes, err := queryReceiver.storageClient.Get(ctx, queryReceiver.trackingValueStorageKey)
	if err != nil || storedTrackingValueBytes == nil {
		return trackingValueFromConfig
	}

	return string(storedTrackingValueBytes)

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
		for _, row := range rows {
			rowToLog(row, logsConfig, scopeLogs.AppendEmpty())
			if logsConfigIndex == 0 {
				errs = multierr.Append(errs, queryReceiver.storeTrackingValue(ctx, row))
			}
		}
	}
	return logs, nil
}

func (queryReceiver *logsQueryReceiver) storeTrackingValue(ctx context.Context, row stringMap) error {
	if queryReceiver.query.TrackingColumn == "" {
		return nil
	}
	queryReceiver.trackingValue = row[queryReceiver.query.TrackingColumn]
	if queryReceiver.storageClient != nil {
		err := queryReceiver.storageClient.Set(ctx, queryReceiver.trackingValueStorageKey, []byte(queryReceiver.trackingValue))
		if err != nil {
			return err
		}
	}
	return nil
}

func rowToLog(row stringMap, config LogsCfg, logRecord plog.LogRecord) {
	logRecord.Body().SetStr(row[config.BodyColumn])
}

func (queryReceiver *logsQueryReceiver) shutdown(_ context.Context) {
}
