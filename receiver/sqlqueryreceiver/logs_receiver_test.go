// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal/metadata"
)

func TestLogsQueryReceiver_Collect(t *testing.T) {
	now := time.Now()

	fakeClient := &sqlquery.FakeDBClient{
		StringMaps: [][]sqlquery.StringMap{
			{{"col1": "42"}, {"col1": "63"}},
		},
	}
	queryReceiver := logsQueryReceiver{
		client: fakeClient,
		query: sqlquery.Query{
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn: "col1",
				},
			},
		},
	}
	logs, err := queryReceiver.collect(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 2, logs.LogRecordCount())

	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, "42", logRecord.Body().Str())
	assert.GreaterOrEqual(t, logRecord.ObservedTimestamp(), pcommon.NewTimestampFromTime(now))

	logRecord = logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1)
	assert.Equal(t, "63", logRecord.Body().Str())
	assert.GreaterOrEqual(t, logRecord.ObservedTimestamp(), pcommon.NewTimestampFromTime(now))

	assert.Equal(t,
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).ObservedTimestamp(),
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).ObservedTimestamp(),
		"Observed timestamps of all log records collected in a single scrape should be equal",
	)
}

func TestLogsQueryReceiver_MissingColumnInResultSet(t *testing.T) {
	fakeClient := &sqlquery.FakeDBClient{
		StringMaps: [][]sqlquery.StringMap{
			{{"col1": "42"}},
		},
	}
	queryReceiver := logsQueryReceiver{
		client: fakeClient,
		query: sqlquery.Query{
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       "expected_body_column",
					AttributeColumns: []string{"expected_column", "expected_column_2"},
				},
			},
		},
	}
	_, err := queryReceiver.collect(t.Context())
	assert.ErrorContains(t, err, "rowToLog: attribute_column 'expected_column' not found in result set")
	assert.ErrorContains(t, err, "rowToLog: attribute_column 'expected_column_2' not found in result set")
	assert.ErrorContains(t, err, "rowToLog: body_column 'expected_body_column' not found in result set")
}

func TestLogsQueryReceiver_BothDatasourceFields(t *testing.T) {
	createReceiver := createLogsReceiverFunc(fakeDBConnect, mkFakeClient)
	ctx := t.Context()
	receiver, err := createReceiver(
		ctx,
		receivertest.NewNopSettings(metadata.Type),
		&Config{
			Config: sqlquery.Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 10 * time.Second,
					InitialDelay:       time.Second,
				},
				Driver:     "mysql",
				DataSource: "my-datasource", // This should be used
				Host:       "localhost",
				Port:       3306,
				Database:   "ignored-database",
				Username:   "ignored-user",
				Password:   "ignored-pass",

				Queries: []sqlquery.Query{{
					SQL: "select * from foo",
					Logs: []sqlquery.LogsCfg{
						{
							BodyColumn: "col1",
						},
					},
				}},
			},
		},
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	require.NoError(t, receiver.Shutdown(ctx))
}

func TestLogsQueryReceiver_UnreferencedNullColumnWarning(t *testing.T) {
	// An unreferenced NULL column should only produce a warning when
	// IgnoreNullValues is false (or unset). When true, no warning is logged.
	// In all cases the log record itself is collected successfully.
	tests := []struct {
		name             string
		ignoreNullValues bool
		expectWarning    bool
	}{
		{name: "default", ignoreNullValues: false, expectWarning: true},
		{name: "explicit_false", ignoreNullValues: false, expectWarning: true},
		{name: "true", ignoreNullValues: true, expectWarning: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col1 := "col1"
			col1Value := "42"
			fakeClient := &sqlquery.FakeDBClient{
				StringMaps: [][]sqlquery.StringMap{
					{{col1: col1Value}},
				},
				// fakeClient.QueryRows will return ErrNullValueWarning on top of the StringMaps
				ErrNullValueWarning: true,
			}

			core, recorded := observer.New(zap.WarnLevel)
			logger := zap.New(core)

			queryReceiver := logsQueryReceiver{
				client: fakeClient,
				query: sqlquery.Query{
					IgnoreNullValues: tt.ignoreNullValues,
					Logs: []sqlquery.LogsCfg{
						{
							BodyColumn:       col1,
							AttributeColumns: []string{col1},
						},
					},
				},
				logger: logger,
			}
			logs, err := queryReceiver.collect(t.Context())
			assert.NoError(t, err)
			assert.Equal(t, 1, logs.LogRecordCount())
			assert.Equal(t, col1Value, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())

			if tt.expectWarning {
				all := recorded.All()
				require.Len(t, all, 1)
				assert.Equal(t, "problems encountered getting log rows", all[0].Message)
				assert.Equal(t, sqlquery.ErrNullValueWarning.Error(), all[0].ContextMap()["error"])
			} else {
				assert.Empty(t, recorded.All(), "expected no warnings when IgnoreNullValues is true")
			}
		})
	}
}

func TestLogsQueryReceiver_NullValue_ReferencedNullColumnStillErrors(t *testing.T) {
	// An error must be returned when a referenced column (BodyColumn) is NULL,
	// regardless of the IgnoreNullValues setting.
	tests := []struct {
		name             string
		ignoreNullValues bool
	}{
		{name: "default", ignoreNullValues: false},
		{name: "explicit_false", ignoreNullValues: false},
		{name: "true", ignoreNullValues: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &sqlquery.FakeDBClient{
				StringMaps: [][]sqlquery.StringMap{
					{{"other_col": "val"}}, // BodyColumn "missing_col" is absent
				},
				ErrNullValueWarning: true,
			}

			queryReceiver := logsQueryReceiver{
				client: fakeClient,
				query: sqlquery.Query{
					IgnoreNullValues: tt.ignoreNullValues,
					Logs: []sqlquery.LogsCfg{
						{
							BodyColumn: "missing_col",
						},
					},
				},
				logger: zap.NewNop(),
			}
			_, err := queryReceiver.collect(t.Context())
			require.Error(t, err)
			assert.ErrorContains(t, err, "body_column 'missing_col' not found in result set")
		})
	}
}

func TestLogsReceiver_InitialDelay(t *testing.T) {
	fakeClient := &sqlquery.FakeDBClient{
		StringMaps: [][]sqlquery.StringMap{
			{{"col1": "42"}},
			{{"col1": "63"}},
		},
	}

	createReceiver := createLogsReceiverFunc(fakeDBConnect, func(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
		return fakeClient
	})

	ctx := t.Context()
	initialDelay := 50 * time.Millisecond
	collectionInterval := 100 * time.Millisecond

	receiver, err := createReceiver(
		ctx,
		receivertest.NewNopSettings(metadata.Type),
		&Config{
			Config: sqlquery.Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: collectionInterval,
					InitialDelay:       initialDelay,
				},
				Driver:     "postgres",
				DataSource: "my-datasource",
				Queries: []sqlquery.Query{{
					SQL: "select * from foo",
					Logs: []sqlquery.LogsCfg{{
						BodyColumn: "col1",
					}},
				}},
			},
		},
		&consumertest.LogsSink{},
	)
	require.NoError(t, err)

	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		_ = receiver.Shutdown(ctx)
	}()

	time.Sleep(initialDelay / 2)
	sink := receiver.(*logsReceiver).nextConsumer.(*consumertest.LogsSink)
	assert.Equal(t, 0, sink.LogRecordCount(), "should not collect before initial delay")

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() >= 1
	}, initialDelay+50*time.Millisecond, 5*time.Millisecond)
}

func TestStatusReportingLogs(t *testing.T) {
	fakeClient := &sqlquery.FakeDBClient{
		StringMaps: [][]sqlquery.StringMap{
			{{"col1": "42"}},
		},
	}
	createReceiver := createLogsReceiverFunc(fakeDBConnect, func(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
		return fakeClient
	})

	ctx := t.Context()
	statusEvents := make(chan *componentstatus.Event, 10)
	host := &statusReporterHost{
		Host: componenttest.NewNopHost(),
		report: func(event *componentstatus.Event) {
			statusEvents <- event
		},
	}

	receiver, err := createReceiver(
		ctx,
		receivertest.NewNopSettings(metadata.Type),
		&Config{
			Config: sqlquery.Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 10 * time.Millisecond,
				},
				Driver:     "postgres",
				DataSource: "my-datasource",
				Queries: []sqlquery.Query{{
					SQL: "select * from foo",
					Logs: []sqlquery.LogsCfg{{
						BodyColumn: "col1",
					}},
				}},
			},
		},
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NoError(t, receiver.Start(ctx, host))

	select {
	case event := <-statusEvents:
		require.Equal(t, componentstatus.StatusOK, event.Status())
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for status event")
	}

	require.NoError(t, receiver.Shutdown(ctx))
}

// blockingDBClient blocks in QueryRows until the provided context is done,
// then returns the context error. It is used to verify that the configured
// timeout cancels a long-running query.
type blockingDBClient struct{}

func (blockingDBClient) QueryRows(ctx context.Context, _ ...any) ([]sqlquery.StringMap, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestLogsReceiver_Timeout(t *testing.T) {
	createReceiver := createLogsReceiverFunc(fakeDBConnect, func(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
		return blockingDBClient{}
	})

	ctx := t.Context()
	statusEvents := make(chan *componentstatus.Event, 10)
	host := &statusReporterHost{
		Host: componenttest.NewNopHost(),
		report: func(event *componentstatus.Event) {
			statusEvents <- event
		},
	}

	receiver, err := createReceiver(
		ctx,
		receivertest.NewNopSettings(metadata.Type),
		&Config{
			Config: sqlquery.Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 10 * time.Millisecond,
					Timeout:            20 * time.Millisecond,
				},
				Driver:     "postgres",
				DataSource: "my-datasource",
				Queries: []sqlquery.Query{{
					SQL: "select * from foo",
					Logs: []sqlquery.LogsCfg{{
						BodyColumn: "col1",
					}},
				}},
			},
		},
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NoError(t, receiver.Start(ctx, host))
	defer func() {
		require.NoError(t, receiver.Shutdown(ctx))
	}()

	// The blocking query never returns on its own, so the only way a status
	// event with an error is produced is if the configured timeout cancels it.
	require.Eventually(t, func() bool {
		select {
		case event := <-statusEvents:
			return event.Status() == componentstatus.StatusRecoverableError &&
				assert.ErrorIs(t, event.Err(), context.DeadlineExceeded)
		default:
			return false
		}
	}, time.Second, 5*time.Millisecond, "expected a recoverable error status caused by the query timeout")
}

func TestLogsQueryReceiver_StoragePersistence(t *testing.T) {
	fakeClient := &sqlquery.FakeDBClient{
		StringMaps: [][]sqlquery.StringMap{
			{{"col1": "42", "id": "1"}, {"col1": "63", "id": "2"}, {"col1": "84", "id": "3"}},
		},
	}
	mockStorage := &mockStorageClient{
		data: make(map[string][]byte),
	}
	queryReceiver := logsQueryReceiver{
		id:     "my-query",
		client: fakeClient,
		query: sqlquery.Query{
			TrackingColumn: "id",
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn: "col1",
				},
			},
		},
		storageClient:           mockStorage,
		trackingValueStorageKey: "my-query.trackingValue",
	}

	logs, err := queryReceiver.collect(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, logs)
	assert.Equal(t, 3, logs.LogRecordCount())

	// Set should not have been called yet
	assert.Equal(t, 0, mockStorage.setCalls)

	// Call commitTrackingValue to persist
	err = queryReceiver.commitTrackingValue(t.Context())
	assert.NoError(t, err)

	// Set should have been called exactly once (for the last row "3")
	assert.Equal(t, 1, mockStorage.setCalls)
	assert.Equal(t, "3", string(mockStorage.data["my-query.trackingValue"]))
}

func TestLogsReceiver_CommitTrackingValueOnConsumeLogs(t *testing.T) {
	fakeClient := &sqlquery.FakeDBClient{
		StringMaps: [][]sqlquery.StringMap{
			{{"col1": "42", "id": "1"}, {"col1": "63", "id": "2"}},
			{{"col1": "42", "id": "1"}, {"col1": "63", "id": "2"}},
		},
	}
	mockStorage := &mockStorageClient{
		data: make(map[string][]byte),
	}

	createReceiver := createLogsReceiverFunc(fakeDBConnect, func(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
		return fakeClient
	})

	ctx := t.Context()
	collectionInterval := 50 * time.Millisecond
	consumer := &mockLogsConsumer{
		Logs: consumertest.NewNop(),
	}

	receiver, err := createReceiver(
		ctx,
		receivertest.NewNopSettings(metadata.Type),
		&Config{
			Config: sqlquery.Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: collectionInterval,
				},
				Driver:     "postgres",
				DataSource: "my-datasource",
				Queries: []sqlquery.Query{{
					SQL:            "select * from foo",
					TrackingColumn: "id",
					Logs: []sqlquery.LogsCfg{{
						BodyColumn: "col1",
					}},
				}},
			},
		},
		consumer,
	)
	require.NoError(t, err)

	logsRecv := receiver.(*logsReceiver)
	logsRecv.storageClient = mockStorage

	err = logsRecv.createQueryReceivers()
	require.NoError(t, err)
	for _, qr := range logsRecv.queryReceivers {
		qr.client = fakeClient
		qr.storageClient = mockStorage
		qr.trackingValueStorageKey = "my-query.trackingValue"
	}

	// 1. Simulate failure in ConsumeLogs: storage should NOT update
	consumer.err = mockError{}
	logsRecv.collect()

	assert.Equal(t, 0, mockStorage.setCalls)

	// 2. Simulate success in ConsumeLogs: storage should update
	consumer.err = nil
	logsRecv.collect()

	assert.Equal(t, 1, mockStorage.setCalls)
	assert.Equal(t, "2", string(mockStorage.data["my-query.trackingValue"]))
}

type mockError struct{}

func (mockError) Error() string {
	return "consumer failed"
}

type mockLogsConsumer struct {
	consumer.Logs
	err error
}

func (m *mockLogsConsumer) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return m.err
}

type mockStorageClient struct {
	storage.Client
	data     map[string][]byte
	setCalls int
}

func (m *mockStorageClient) Get(_ context.Context, key string) ([]byte, error) {
	return m.data[key], nil
}

func (m *mockStorageClient) Set(_ context.Context, key string, val []byte) error {
	m.setCalls++
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[key] = val
	return nil
}

func (m *mockStorageClient) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (*mockStorageClient) Close(_ context.Context) error {
	return nil
}
