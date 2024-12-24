// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package sqlqueryreceiver

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	postgresqlPort = "5432"
	oraclePort     = "1521"
	mysqlPort      = "3306"
	sqlServerPort  = "1433"
)

type DbEngine struct {
	Port                     string
	SqlParameter             string
	CheckCompatibility       func(t *testing.T)
	ConnectionString         func(host string, externalPort nat.Port) string
	Driver                   string
	CurrentTimestampFunction string
	ConvertColumnName        func(string) string
	ContainerRequest         testcontainers.ContainerRequest
}

var (
	Postgres = DbEngine{
		Port:         postgresqlPort,
		SqlParameter: "$1",
		CheckCompatibility: func(t *testing.T) {
			// No compatibility checks needed for Postgres
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("host=%s port=%s user=otel password=otel sslmode=disable", host, externalPort.Port())
		},
		Driver:                   "postgres",
		CurrentTimestampFunction: "now()",
		ConvertColumnName:        func(name string) string { return name },
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:9.6.24",
			Env: map[string]string{
				"POSTGRES_USER":     "root",
				"POSTGRES_PASSWORD": "otel",
				"POSTGRES_DB":       "otel",
			},
			Files: []testcontainers.ContainerFile{{
				HostFilePath:      filepath.Join("testdata", "integration", "postgresql", "init.sql"),
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
				FileMode:          700,
			}},
			ExposedPorts: []string{postgresqlPort},
			WaitingFor: wait.ForListeningPort(postgresqlPort).
				WithStartupTimeout(2 * time.Minute),
		},
	}
	MySql = DbEngine{
		Port:         mysqlPort,
		SqlParameter: "?",
		CheckCompatibility: func(t *testing.T) {
			// No compatibility checks needed for MySql
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("otel:otel@tcp(%s:%s)/otel", host, externalPort.Port())
		},
		Driver:                   "mysql",
		CurrentTimestampFunction: "now()",
		ConvertColumnName:        func(name string) string { return name },
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "mysql:8.0.33",
			Env: map[string]string{
				"MYSQL_USER":          "otel",
				"MYSQL_PASSWORD":      "otel",
				"MYSQL_ROOT_PASSWORD": "otel",
				"MYSQL_DATABASE":      "otel",
			},
			Files: []testcontainers.ContainerFile{{
				HostFilePath:      filepath.Join("testdata", "integration", "mysql", "init.sql"),
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
				FileMode:          700,
			}},
			ExposedPorts: []string{mysqlPort},
			WaitingFor:   wait.ForListeningPort(mysqlPort).WithStartupTimeout(2 * time.Minute),
		},
	}
	Oracle = DbEngine{
		Port:         oraclePort,
		SqlParameter: ":1",
		CheckCompatibility: func(t *testing.T) {
			if runtime.GOARCH == "arm64" {
				t.Skip("Incompatible with arm64")
			}
			t.Skip("Skipping the test until https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27577 is fixed")
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("oracle://otel:p@ssw%%25rd@%s:%s/XE", host, externalPort.Port())
		},
		Driver:                   "oracle",
		CurrentTimestampFunction: "SYSTIMESTAMP",
		ConvertColumnName:        strings.ToUpper,
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration", "oracle"),
				Dockerfile: "Dockerfile.oracledb",
			},
			ExposedPorts: []string{oraclePort},
			WaitingFor:   wait.NewHealthStrategy().WithStartupTimeout(30 * time.Minute),
		},
	}
	SqlServer = DbEngine{
		Port:         sqlServerPort,
		SqlParameter: "@p1",
		CheckCompatibility: func(t *testing.T) {
			if runtime.GOARCH == "arm64" {
				t.Skip("Incompatible with arm64")
			}
			t.Skip("Skipping the test until https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27577 is fixed")
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("sqlserver://otel:YourStrong%%21Passw0rd@%s:%s?database=otel", host, externalPort.Port())
		},
		Driver:                   "sqlserver",
		CurrentTimestampFunction: "GETDATE()",
		ConvertColumnName:        func(name string) string { return name },
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration", "sqlserver"),
				Dockerfile: "Dockerfile",
			},
			Env: map[string]string{
				"ACCEPT_EULA": "Y",
				"SA_PASSWORD": "YourStrong!Passw0rd",
			},
			ExposedPorts: []string{sqlServerPort},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort(sqlServerPort),
				wait.ForLog("Initiation of otel database is complete"),
			).WithDeadline(5 * time.Minute),
		},
	}
)

func TestIntegrationLogsTrackingWithStorage(t *testing.T) {
	tests := []struct {
		name   string
		engine DbEngine
	}{
		{name: "Postgres", engine: Postgres},
		{name: "MySQL", engine: MySql},
		{name: "SqlServer", engine: SqlServer},
		{name: "Oracle", engine: Oracle},
	}

	for _, tt := range tests {
		trackingColumn := tt.engine.ConvertColumnName("id")
		trackingStartValue := "0"
		t.Run(tt.name, func(t *testing.T) {
			tt.engine.CheckCompatibility(t)
			dbContainer, dbHost, externalPort := startDbContainerWithConfig(t, tt.engine)
			defer func() {
				require.NoError(t, dbContainer.Terminate(context.Background()))
			}()

			storageDir := t.TempDir()
			storageExtension := storagetest.NewFileBackedStorageExtension("test", storageDir)

			receiverCreateSettings := receivertest.NewNopSettings()
			receiver, config, consumer := createTestLogsReceiver(t, tt.engine.Driver, tt.engine.ConnectionString(dbHost, externalPort), receiverCreateSettings)
			config.CollectionInterval = time.Second
			config.Telemetry.Logs.Query = true
			config.StorageID = &storageExtension.ID
			config.Queries = []sqlquery.Query{
				{
					SQL: fmt.Sprintf("select * from simple_logs where %s > %s", trackingColumn, tt.engine.SqlParameter),
					Logs: []sqlquery.LogsCfg{
						{
							BodyColumn:       tt.engine.ConvertColumnName("body"),
							AttributeColumns: []string{tt.engine.ConvertColumnName("attribute")},
						},
					},
					TrackingColumn:     trackingColumn,
					TrackingStartValue: trackingStartValue,
				},
			}

			host := storagetest.NewStorageHost().WithExtension(storageExtension.ID, storageExtension)
			err := receiver.Start(context.Background(), host)
			require.NoError(t, err)

			require.Eventuallyf(
				t,
				func() bool {
					return consumer.LogRecordCount() > 0
				},
				1*time.Minute,
				1*time.Second,
				"failed to receive more than 0 logs",
			)

			err = receiver.Shutdown(context.Background())
			require.NoError(t, err)

			initialLogCount := 5
			require.Equal(t, initialLogCount, consumer.LogRecordCount())
			testAllSimpleLogs(t, consumer.AllLogs(), tt.engine.ConvertColumnName("attribute"))

			receiver, config, consumer = createTestLogsReceiver(t, tt.engine.Driver, tt.engine.ConnectionString(dbHost, externalPort), receiverCreateSettings)
			config.CollectionInterval = time.Second
			config.Telemetry.Logs.Query = true
			config.StorageID = &storageExtension.ID
			config.Queries = []sqlquery.Query{
				{
					SQL: fmt.Sprintf("select * from simple_logs where %s > %s", trackingColumn, tt.engine.SqlParameter),
					Logs: []sqlquery.LogsCfg{
						{
							BodyColumn:       tt.engine.ConvertColumnName("body"),
							AttributeColumns: []string{tt.engine.ConvertColumnName("attribute")},
						},
					},
					TrackingColumn:     trackingColumn,
					TrackingStartValue: trackingStartValue,
				},
			}
			err = receiver.Start(context.Background(), host)
			require.NoError(t, err)

			time.Sleep(5 * time.Second)

			err = receiver.Shutdown(context.Background())
			require.NoError(t, err)

			require.Equal(t, 0, consumer.LogRecordCount())

			newLogCount := 3
			insertSimpleLogs(t, tt.engine, dbContainer, initialLogCount, newLogCount)

			receiver, config, consumer = createTestLogsReceiver(t, tt.engine.Driver, tt.engine.ConnectionString(dbHost, externalPort), receiverCreateSettings)
			config.CollectionInterval = time.Second
			config.Telemetry.Logs.Query = true
			config.StorageID = &storageExtension.ID
			config.Queries = []sqlquery.Query{
				{
					SQL: fmt.Sprintf("select * from simple_logs where %s > %s", trackingColumn, tt.engine.SqlParameter),
					Logs: []sqlquery.LogsCfg{
						{
							BodyColumn:       tt.engine.ConvertColumnName("body"),
							AttributeColumns: []string{tt.engine.ConvertColumnName("attribute")},
						},
					},
					TrackingColumn:     trackingColumn,
					TrackingStartValue: trackingStartValue,
				},
			}
			err = receiver.Start(context.Background(), host)
			require.NoError(t, err)

			require.Eventuallyf(
				t,
				func() bool {
					return consumer.LogRecordCount() > 0
				},
				1*time.Minute,
				1*time.Second,
				"failed to receive more than 0 logs",
			)

			err = receiver.Shutdown(context.Background())
			require.NoError(t, err)

			require.Equal(t, newLogCount, consumer.LogRecordCount())
		})
	}
}

func TestIntegrationLogsTrackingWithoutStorage(t *testing.T) {
	tests := []struct {
		name                     string
		engine                   DbEngine
		trackingColumn           string
		trackingStartValue       string
		trackingStartValueFormat string
	}{
		{name: "PostgresById", engine: Postgres, trackingColumn: "id", trackingStartValue: "0", trackingStartValueFormat: ""},
		{name: "MySQLById", engine: MySql, trackingColumn: "id", trackingStartValue: "0", trackingStartValueFormat: ""},
		{name: "SqlServerById", engine: SqlServer, trackingColumn: "id", trackingStartValue: "0", trackingStartValueFormat: ""},
		{name: "OracleById", engine: Oracle, trackingColumn: "ID", trackingStartValue: "0", trackingStartValueFormat: ""},
		{name: "PostgresByTimestamp", engine: Postgres, trackingColumn: "insert_time", trackingStartValue: "2022-06-03 21:00:00+00", trackingStartValueFormat: ""},
		{name: "MySQLByTimestamp", engine: MySql, trackingColumn: "insert_time", trackingStartValue: "2022-06-03 21:00:00", trackingStartValueFormat: ""},
		{name: "SqlServerByTimestamp", engine: SqlServer, trackingColumn: "insert_time", trackingStartValue: "2022-06-03 21:00:00", trackingStartValueFormat: ""},
		{name: "OracleByTimestamp", engine: Oracle, trackingColumn: "INSERT_TIME", trackingStartValue: "2022-06-03T21:00:00.000Z", trackingStartValueFormat: "TO_TIMESTAMP_TZ(:1, 'YYYY-MM-DD\"T\"HH24:MI:SS.FF6TZH:TZM')"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.engine.CheckCompatibility(t)
			dbContainer, dbHost, externalPort := startDbContainerWithConfig(t, tt.engine)
			defer func() {
				require.NoError(t, dbContainer.Terminate(context.Background()))
			}()

			receiverCreateSettings := receivertest.NewNopSettings()
			receiver, config, consumer := createTestLogsReceiver(t, tt.engine.Driver, tt.engine.ConnectionString(dbHost, externalPort), receiverCreateSettings)
			config.CollectionInterval = 100 * time.Millisecond
			config.Telemetry.Logs.Query = true

			trackingColumn := tt.engine.ConvertColumnName(tt.trackingColumn)
			trackingColumnParameter := tt.engine.SqlParameter
			if tt.trackingStartValueFormat != "" {
				trackingColumnParameter = tt.trackingStartValueFormat
			}

			config.Queries = []sqlquery.Query{
				{
					SQL: fmt.Sprintf("select * from simple_logs where %s > %s order by %s asc", trackingColumn, trackingColumnParameter, trackingColumn),
					Logs: []sqlquery.LogsCfg{
						{
							BodyColumn:       tt.engine.ConvertColumnName("body"),
							AttributeColumns: []string{tt.engine.ConvertColumnName("attribute")},
						},
					},
					TrackingColumn:     trackingColumn,
					TrackingStartValue: tt.trackingStartValue,
				},
			}
			host := componenttest.NewNopHost()
			err := receiver.Start(context.Background(), host)
			require.NoError(t, err)

			require.Eventuallyf(
				t,
				func() bool {
					return consumer.LogRecordCount() > 0
				},
				1*time.Minute,
				500*time.Millisecond,
				"failed to receive more than 0 logs",
			)
			require.Equal(t, 5, consumer.LogRecordCount())
			testAllSimpleLogs(t, consumer.AllLogs(), tt.engine.ConvertColumnName("attribute"))

			err = receiver.Shutdown(context.Background())
			require.NoError(t, err)
		})
	}
}

func startDbContainerWithConfig(t *testing.T, engine DbEngine) (testcontainers.Container, string, nat.Port) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: engine.ContainerRequest,
			Started:          true,
		},
	)
	require.NoError(t, err)
	dbPort, err := container.MappedPort(ctx, nat.Port(engine.Port))
	require.NoError(t, err)
	host, err := container.Host(ctx)
	require.NoError(t, err)
	return container, host, dbPort
}

func insertSimpleLogs(t *testing.T, engine DbEngine, container testcontainers.Container, existingLogID, newLogCount int) {
	externalPort, err := container.MappedPort(context.Background(), nat.Port(engine.Port))
	require.NoError(t, err)

	host, err := container.Host(context.Background())
	require.NoError(t, err)

	db, err := sql.Open(engine.Driver, engine.ConnectionString(host, externalPort))
	require.NoError(t, err)
	defer db.Close()

	for newLogID := existingLogID + 1; newLogID <= existingLogID+newLogCount; newLogID++ {
		query := fmt.Sprintf("insert into simple_logs (id, insert_time, body, attribute) values (%d, %s, 'another log %d', 'TLSv1.2')", newLogID, engine.CurrentTimestampFunction, newLogID) // nolint:gosec // Ignore, not possible to use prepared statements here for currentTimestampFunction
		_, err := db.Exec(query)
		require.NoError(t, err)
	}
}

func createTestLogsReceiver(t *testing.T, driver, dataSource string, receiverCreateSettings receiver.Settings) (*logsReceiver, *Config, *consumertest.LogsSink) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Driver = driver
	config.DataSource = dataSource

	consumer := &consumertest.LogsSink{}
	receiverCreateSettings.Logger = zap.NewExample()
	receiver, err := factory.CreateLogs(
		context.Background(),
		receiverCreateSettings,
		config,
		consumer,
	)
	require.NoError(t, err)
	return receiver.(*logsReceiver), config, consumer
}

func testAllSimpleLogs(t *testing.T, logs []plog.Logs, attributeColumnName string) {
	assert.Len(t, logs, 1)
	assert.Equal(t, 1, logs[0].ResourceLogs().Len())
	assert.Equal(t, 1, logs[0].ResourceLogs().At(0).ScopeLogs().Len())
	expectedLogBodies := []string{
		"- - - [03/Jun/2022:21:59:26 +0000] \"GET /api/health HTTP/1.1\" 200 6197 4 \"-\" \"-\" 445af8e6c428303f -",
		"- - - [03/Jun/2022:21:59:26 +0000] \"GET /api/health HTTP/1.1\" 200 6205 5 \"-\" \"-\" 3285f43cd4baa202 -",
		"- - - [03/Jun/2022:21:59:29 +0000] \"GET /api/health HTTP/1.1\" 200 6233 4 \"-\" \"-\" 579e8362d3185b61 -",
		"- - - [03/Jun/2022:21:59:31 +0000] \"GET /api/health HTTP/1.1\" 200 6207 5 \"-\" \"-\" 8c6ac61ae66e509f -",
		"- - - [03/Jun/2022:21:59:31 +0000] \"GET /api/health HTTP/1.1\" 200 6200 4 \"-\" \"-\" c163495861e873d8 -",
	}
	expectedLogAttributes := []string{
		"TLSv1.2",
		"TLSv1",
		"TLSv1.2",
		"TLSv1",
		"TLSv1.2",
	}
	assert.Equal(t, len(expectedLogBodies), logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	for i := range expectedLogBodies {
		logRecord := logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(i)
		assert.Equal(t, expectedLogBodies[i], logRecord.Body().Str())
		logAttribute, _ := logRecord.Attributes().Get(attributeColumnName)
		assert.Equal(t, expectedLogAttributes[i], logAttribute.Str())
	}
}

func TestPostgresqlIntegrationMetrics(t *testing.T) {
	Postgres.CheckCompatibility(t)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			Postgres.ContainerRequest),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = Postgres.Driver
				rCfg.DataSource = Postgres.ConnectionString(ci.Host(t), nat.Port(ci.MappedPort(t, Postgres.Port)))
				rCfg.Queries = []sqlquery.Query{
					{
						SQL: "select genre, count(*), avg(imdb_rating) from movie group by genre",
						Metrics: []sqlquery.MetricCfg{
							{
								MetricName:       "genre.count",
								ValueColumn:      "count",
								AttributeColumns: []string{"genre"},
								ValueType:        sqlquery.MetricValueTypeInt,
								DataType:         sqlquery.MetricTypeGauge,
							},
							{
								MetricName:       "genre.imdb",
								ValueColumn:      "avg",
								AttributeColumns: []string{"genre"},
								ValueType:        sqlquery.MetricValueTypeDouble,
								DataType:         sqlquery.MetricTypeGauge,
							},
						},
					},
					{
						SQL: "select 1::smallint as a, 2::integer as b, 3::bigint as c, 4.1::decimal as d," +
							" 4.2::numeric as e, 4.3::real as f, 4.4::double precision as g, null as h",
						Metrics: []sqlquery.MetricCfg{
							{
								MetricName:  "a",
								ValueColumn: "a",
								ValueType:   sqlquery.MetricValueTypeInt,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "b",
								ValueColumn: "b",
								ValueType:   sqlquery.MetricValueTypeInt,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "c",
								ValueColumn: "c",
								ValueType:   sqlquery.MetricValueTypeInt,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "d",
								ValueColumn: "d",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "e",
								ValueColumn: "e",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "f",
								ValueColumn: "f",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "g",
								ValueColumn: "g",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "h",
								ValueColumn: "h",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
						},
					},
				}
			}),
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", "postgresql", "expected.yaml"),
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}

// This test ensures the collector can connect to an Oracle DB, and properly get metrics. It's not intended to
// test the receiver itself.
func TestOracleDBIntegrationMetrics(t *testing.T) {
	Oracle.CheckCompatibility(t)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			Oracle.ContainerRequest),
		scraperinttest.WithCreateContainerTimeout(30*time.Minute),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = Oracle.Driver
				rCfg.DataSource = Oracle.ConnectionString(ci.Host(t), nat.Port(ci.MappedPort(t, Oracle.Port)))
				rCfg.Queries = []sqlquery.Query{
					{
						SQL: "select genre, count(*) as count, avg(imdb_rating) as avg from movie group by genre",
						Metrics: []sqlquery.MetricCfg{
							{
								MetricName:       "genre.count",
								ValueColumn:      "COUNT",
								AttributeColumns: []string{"GENRE"},
								ValueType:        sqlquery.MetricValueTypeInt,
								DataType:         sqlquery.MetricTypeGauge,
							},
							{
								MetricName:       "genre.imdb",
								ValueColumn:      "AVG",
								AttributeColumns: []string{"GENRE"},
								ValueType:        sqlquery.MetricValueTypeDouble,
								DataType:         sqlquery.MetricTypeGauge,
							},
						},
					},
				}
			}),
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", "oracle", "expected.yaml"),
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}

func TestMysqlIntegrationMetrics(t *testing.T) {
	MySql.CheckCompatibility(t)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(MySql.ContainerRequest),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = MySql.Driver
				rCfg.DataSource = MySql.ConnectionString(ci.Host(t), nat.Port(ci.MappedPort(t, MySql.Port)))
				rCfg.Queries = []sqlquery.Query{
					{
						SQL: "select genre, count(*), avg(imdb_rating) from movie group by genre order by genre desc",
						Metrics: []sqlquery.MetricCfg{
							{
								MetricName:       "genre.count",
								ValueColumn:      "count(*)",
								AttributeColumns: []string{"genre"},
								ValueType:        sqlquery.MetricValueTypeInt,
								DataType:         sqlquery.MetricTypeGauge,
							},
							{
								MetricName:       "genre.imdb",
								ValueColumn:      "avg(imdb_rating)",
								AttributeColumns: []string{"genre"},
								ValueType:        sqlquery.MetricValueTypeDouble,
								DataType:         sqlquery.MetricTypeGauge,
							},
						},
					},
					{
						SQL: "select " +
							"cast(1 as signed) as a, " +
							"cast(2 as unsigned) as b, " +
							"cast(3.1 as decimal(10,1)) as c, " +
							"cast(3.2 as real) as d, " +
							"cast(3.3 as float) as e, " +
							"cast(3.4 as double) as f, " +
							"null as g",
						Metrics: []sqlquery.MetricCfg{
							{
								MetricName:  "a",
								ValueColumn: "a",
								ValueType:   sqlquery.MetricValueTypeInt,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "b",
								ValueColumn: "b",
								ValueType:   sqlquery.MetricValueTypeInt,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "c",
								ValueColumn: "c",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "d",
								ValueColumn: "d",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "e",
								ValueColumn: "e",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
							{
								MetricName:  "f",
								ValueColumn: "f",
								ValueType:   sqlquery.MetricValueTypeDouble,
								DataType:    sqlquery.MetricTypeGauge,
							},
						},
					},
				}
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", "mysql", "expected.yaml")),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}

func TestSqlServerIntegrationMetrics(t *testing.T) {
	SqlServer.CheckCompatibility(t)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(SqlServer.ContainerRequest),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = SqlServer.Driver
				rCfg.DataSource = SqlServer.ConnectionString(ci.Host(t), nat.Port(ci.MappedPort(t, SqlServer.Port)))
				rCfg.Queries = []sqlquery.Query{
					{
						SQL: "select genre, count(*) as count, avg(imdb_rating) as avg from movie group by genre order by genre",
						Metrics: []sqlquery.MetricCfg{
							{
								MetricName:       "genre.count",
								ValueColumn:      "count",
								AttributeColumns: []string{"genre"},
								ValueType:        sqlquery.MetricValueTypeInt,
								DataType:         sqlquery.MetricTypeGauge,
							},
							{
								MetricName:       "genre.imdb",
								ValueColumn:      "avg",
								AttributeColumns: []string{"genre"},
								ValueType:        sqlquery.MetricValueTypeDouble,
								DataType:         sqlquery.MetricTypeGauge,
							},
						},
					},
				}
			}),
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", "sqlserver", "expected.yaml"),
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreMetricsOrder(),
		),
	).Run(t)
}
