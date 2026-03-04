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

func TestLogsQueryReceiver_NullValue(t *testing.T) {
	col1 := "col1"
	col1Value := "42"
	fakeClient := &sqlquery.FakeDBClient{
		StringMaps: [][]sqlquery.StringMap{
			{{col1: col1Value}},
		},
		// fakeClient.QueryRows will return ErrNullValueWarning on top of the StringMaps
		ErrNullValueWarning: true,
	}

	core, recorded := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	queryReceiver := logsQueryReceiver{
		client: fakeClient,
		query: sqlquery.Query{
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       col1,
					AttributeColumns: []string{col1},
				},
			},
		},
		logger: logger,
	}
	// ensure that the logs are collected successfully
	logs, err := queryReceiver.collect(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 1, logs.LogRecordCount())
	assert.Equal(t, col1Value, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())

	// ensure that the warning is logged
	all := recorded.All()
	require.Len(t, all, 1)
	entry := all[0]
	require.Equal(t, "problems encountered getting log rows", entry.Message)
	require.Equal(t, sqlquery.ErrNullValueWarning.Error(), entry.ContextMap()["error"])
}
