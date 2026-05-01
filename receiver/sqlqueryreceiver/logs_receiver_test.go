// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
