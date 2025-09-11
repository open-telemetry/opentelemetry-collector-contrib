// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal/metadata"
)

func TestCreateLogs(t *testing.T) {
	createReceiver := createLogsReceiverFunc(fakeDBConnect, mkFakeClient)
	ctx := t.Context()
	receiver, err := createReceiver(
		ctx,
		receivertest.NewNopSettings(metadata.Type),
		&Config{
			Config: sqlquery.Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 10 * time.Second,
				},
				Driver:     "postgres",
				DataSource: "my-datasource",
				Queries: []sqlquery.Query{{
					SQL: "select * from foo",
					Logs: []sqlquery.LogsCfg{
						{},
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

func TestCreateMetrics(t *testing.T) {
	createReceiver := createMetricsReceiverFunc(fakeDBConnect, mkFakeClient)
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
				Driver:     "postgres",
				DataSource: "my-datasource",
				Queries: []sqlquery.Query{{
					SQL: "select * from foo",
					Metrics: []sqlquery.MetricCfg{{
						MetricName:  "my-metric",
						ValueColumn: "my-column",
					}},
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

func TestCreateLogsDatasourceFields(t *testing.T) {
	createReceiver := createLogsReceiverFunc(fakeDBConnect, mkFakeClient)
	ctx := t.Context()
	receiver, err := createReceiver(
		ctx,
		receivertest.NewNopSettings(metadata.Type),
		&Config{
			Config: sqlquery.Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 10 * time.Second,
				},
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "my-datasource",
				Username: "user",
				Password: "pass",
				Queries: []sqlquery.Query{{
					SQL: "select * from foo",
					Logs: []sqlquery.LogsCfg{
						{},
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

func TestCreateMetricsDatasourceFields(t *testing.T) {
	createReceiver := createMetricsReceiverFunc(fakeDBConnect, mkFakeClient)
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
				Driver:   "mysql",
				Host:     "localhost",
				Port:     3306,
				Database: "my-datasource",
				Username: "user",
				Password: "pass",
				Queries: []sqlquery.Query{{
					SQL: "select * from foo",
					Metrics: []sqlquery.MetricCfg{{
						MetricName:  "my-metric",
						ValueColumn: "my-column",
					}},
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

func TestCreateMetricsBothDatasourceFields(t *testing.T) {
	createReceiver := createMetricsReceiverFunc(fakeDBConnect, mkFakeClient)
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
					Metrics: []sqlquery.MetricCfg{{
						MetricName:  "my-metric",
						ValueColumn: "my-column",
					}},
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

func fakeDBConnect(string, string) (*sql.DB, error) {
	return nil, nil
}

func mkFakeClient(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
	return &sqlquery.FakeDBClient{StringMaps: [][]sqlquery.StringMap{{{"foo": "111"}}}}
}
