// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal/metadata"
)

type logsReceiver struct {
	config           *Config
	settings         receiver.Settings
	createConnection sqlquery.DbProviderFunc
	createClient     sqlquery.ClientProviderFunc
	queryReceivers   []*logsQueryReceiver
	nextConsumer     consumer.Logs

	isStarted                bool
	collectionIntervalTicker *time.Ticker
	shutdownRequested        chan struct{}

	id            component.ID
	storageClient storage.Client
	obsrecv       *receiverhelper.ObsReport
}

func newLogsReceiver(
	config *Config,
	settings receiver.Settings,
	sqlOpenerFunc sqlquery.SQLOpenerFunc,
	createClient sqlquery.ClientProviderFunc,
	nextConsumer consumer.Logs,
) (*logsReceiver, error) {

	obsr, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
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
			receiver.config.Telemetry,
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
		receiver.obsrecv.EndLogsOp(ctx, metadata.Type.String(), logRecordCount, err)
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

	var errs []error
	receiver.settings.Logger.Debug("stopping...")
	receiver.stopCollecting()
	for _, queryReceiver := range receiver.queryReceivers {
		errs = append(errs, queryReceiver.shutdown(ctx))
	}

	if receiver.storageClient != nil {
		errs = append(errs, receiver.storageClient.Close(ctx))
	}

	receiver.isStarted = false
	receiver.settings.Logger.Debug("stopped.")

	return errors.Join(errs...)
}

func (receiver *logsReceiver) stopCollecting() {
	if receiver.collectionIntervalTicker != nil {
		receiver.collectionIntervalTicker.Stop()
	}
	close(receiver.shutdownRequested)
}

type logsQueryReceiver struct {
	id           string
	query        sqlquery.Query
	createDb     sqlquery.DbProviderFunc
	createClient sqlquery.ClientProviderFunc
	logger       *zap.Logger
	telemetry    sqlquery.TelemetryConfig

	db            *sql.DB
	client        sqlquery.DbClient
	trackingValue string
	// TODO: Extract persistence into its own component
	storageClient           storage.Client
	trackingValueStorageKey string
}

func newLogsQueryReceiver(
	id string,
	query sqlquery.Query,
	dbProviderFunc sqlquery.DbProviderFunc,
	clientProviderFunc sqlquery.ClientProviderFunc,
	logger *zap.Logger,
	telemetry sqlquery.TelemetryConfig,
	storageClient storage.Client,
) *logsQueryReceiver {
	queryReceiver := &logsQueryReceiver{
		id:            id,
		query:         query,
		createDb:      dbProviderFunc,
		createClient:  clientProviderFunc,
		logger:        logger,
		telemetry:     telemetry,
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
	queryReceiver.client = queryReceiver.createClient(sqlquery.DbWrapper{Db: queryReceiver.db}, queryReceiver.query.SQL, queryReceiver.logger, queryReceiver.telemetry)

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

	var rows []sqlquery.StringMap
	var err error
	observedAt := pcommon.NewTimestampFromTime(time.Now())
	if queryReceiver.query.TrackingColumn != "" {
		rows, err = queryReceiver.client.QueryRows(ctx, queryReceiver.trackingValue)
	} else {
		rows, err = queryReceiver.client.QueryRows(ctx)
	}
	if err != nil {
		return logs, fmt.Errorf("error getting rows: %w", err)
	}

	var errs []error
	scopeLogs := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for logsConfigIndex, logsConfig := range queryReceiver.query.Logs {
		for _, row := range rows {
			logRecord := scopeLogs.AppendEmpty()
			errs = append(errs, rowToLog(row, logsConfig, logRecord))
			logRecord.SetObservedTimestamp(observedAt)
			if logsConfigIndex == 0 {
				errs = append(errs, queryReceiver.storeTrackingValue(ctx, row))
			}
		}
	}
	return logs, errors.Join(errs...)
}

func (queryReceiver *logsQueryReceiver) storeTrackingValue(ctx context.Context, row sqlquery.StringMap) error {
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

func rowToLog(row sqlquery.StringMap, config sqlquery.LogsCfg, logRecord plog.LogRecord) error {
	var errs []error
	value, found := row[config.BodyColumn]
	if !found {
		errs = append(errs, fmt.Errorf("rowToLog: body_column '%s' not found in result set", config.BodyColumn))
	} else {
		logRecord.Body().SetStr(value)
	}
	attrs := logRecord.Attributes()

	for _, columnName := range config.AttributeColumns {
		if attrVal, found := row[columnName]; found {
			attrs.PutStr(columnName, attrVal)
		} else {
			errs = append(errs, fmt.Errorf("rowToLog: attribute_column '%s' not found in result set", columnName))
		}
	}
	return errors.Join(errs...)
}

func (queryReceiver *logsQueryReceiver) shutdown(_ context.Context) error {
	if queryReceiver.db == nil {
		return nil
	}

	return queryReceiver.db.Close()
}
