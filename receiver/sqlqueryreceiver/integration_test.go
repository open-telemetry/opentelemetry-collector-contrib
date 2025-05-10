// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package sqlqueryreceiver

import (
	"context"
	"fmt"
	"io"
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
)

func TestPostgresIntegrationLogsTrackingWithoutStorage(t *testing.T) {
	// Start Postgres container.
	externalPort := "15430"
	dbContainer := startPostgresDbContainer(t, externalPort)
	defer func() {
		require.NoError(t, dbContainer.Terminate(context.Background()))
	}()

	// Start the SQL Query receiver.
	receiverCreateSettings := receivertest.NewNopSettings()
	receiver, config, consumer := createTestLogsReceiverForPostgres(t, externalPort, receiverCreateSettings)
	config.CollectionInterval = time.Second
	config.Queries = []sqlquery.Query{
		{
			SQL: "select * from simple_logs where id > $1",
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       "body",
					AttributeColumns: []string{"attribute"},
				},
			},
			TrackingColumn:     "id",
			TrackingStartValue: "0",
		},
	}
	host := componenttest.NewNopHost()
	err := receiver.Start(context.Background(), host)
	require.NoError(t, err)

	// Verify there's 5 logs received.
	require.Eventuallyf(
		t,
		func() bool {
			return consumer.LogRecordCount() > 0
		},
		1*time.Minute,
		1*time.Second,
		"failed to receive more than 0 logs",
	)
	require.Equal(t, 5, consumer.LogRecordCount())
	testAllSimpleLogs(t, consumer.AllLogs())

	// Stop the SQL Query receiver.
	err = receiver.Shutdown(context.Background())
	require.NoError(t, err)

	// Start new SQL Query receiver with the same configuration.
	receiver, config, consumer = createTestLogsReceiverForPostgres(t, externalPort, receiverCreateSettings)
	config.CollectionInterval = time.Second
	config.Queries = []sqlquery.Query{
		{
			SQL: "select * from simple_logs where id > $1",
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       "body",
					AttributeColumns: []string{"attribute"},
				},
			},
			TrackingColumn:     "id",
			TrackingStartValue: "0",
		},
	}
	err = receiver.Start(context.Background(), host)
	require.NoError(t, err)

	// Wait for some logs to come in.
	require.Eventuallyf(
		t,
		func() bool {
			return consumer.LogRecordCount() > 0
		},
		1*time.Minute,
		1*time.Second,
		"failed to receive more than 0 logs",
	)

	// stop the SQL Query receiver
	err = receiver.Shutdown(context.Background())
	require.NoError(t, err)

	// Verify that the same logs are collected again.
	require.Equal(t, 5, consumer.LogRecordCount())
	testAllSimpleLogs(t, consumer.AllLogs())
}

func TestPostgresIntegrationLogsTrackingByTimestampColumnWithoutStorage(t *testing.T) {
	// Start Postgres container.
	externalPort := "15432"
	dbContainer := startPostgresDbContainer(t, externalPort)
	defer func() {
		require.NoError(t, dbContainer.Terminate(context.Background()))
	}()

	// Start the SQL Query receiver.
	receiverCreateSettings := receivertest.NewNopSettings()
	receiver, config, consumer := createTestLogsReceiverForPostgres(t, externalPort, receiverCreateSettings)
	config.CollectionInterval = 100 * time.Millisecond
	config.Queries = []sqlquery.Query{
		{
			SQL: "select * from simple_logs where insert_time > $1 order by insert_time asc",
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       "body",
					AttributeColumns: []string{"attribute"},
				},
			},
			TrackingColumn:     "insert_time",
			TrackingStartValue: "2022-06-03 21:00:00+00",
		},
	}
	host := componenttest.NewNopHost()
	err := receiver.Start(context.Background(), host)
	require.NoError(t, err)

	// Verify there's 5 logs received.
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
	testAllSimpleLogs(t, consumer.AllLogs())

	// Stop the SQL Query receiver.
	err = receiver.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPostgresIntegrationLogsTrackingWithStorage(t *testing.T) {
	// start Postgres container
	externalPort := "15431"
	dbContainer := startPostgresDbContainer(t, externalPort)
	defer func() {
		require.NoError(t, dbContainer.Terminate(context.Background()))
	}()

	// create a File Storage extension writing to a temporary directory in local filesystem
	storageDir := t.TempDir()
	storageExtension := storagetest.NewFileBackedStorageExtension("test", storageDir)

	// create SQL Query receiver configured with the File Storage extension
	receiverCreateSettings := receivertest.NewNopSettings()
	receiver, config, consumer := createTestLogsReceiverForPostgres(t, externalPort, receiverCreateSettings)
	config.CollectionInterval = time.Second
	config.StorageID = &storageExtension.ID
	config.Queries = []sqlquery.Query{
		{
			SQL: "select * from simple_logs where id > $1",
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       "body",
					AttributeColumns: []string{"attribute"},
				},
			},
			TrackingColumn:     "id",
			TrackingStartValue: "0",
		},
	}

	// start the SQL Query receiver
	host := storagetest.NewStorageHost().WithExtension(storageExtension.ID, storageExtension)
	err := receiver.Start(context.Background(), host)
	require.NoError(t, err)

	// Wait for logs to come in.
	require.Eventuallyf(
		t,
		func() bool {
			return consumer.LogRecordCount() > 0
		},
		1*time.Minute,
		1*time.Second,
		"failed to receive more than 0 logs",
	)

	// stop the SQL Query receiver
	err = receiver.Shutdown(context.Background())
	require.NoError(t, err)

	// verify there's 5 logs received
	initialLogCount := 5
	require.Equal(t, initialLogCount, consumer.LogRecordCount())
	testAllSimpleLogs(t, consumer.AllLogs())

	// start the SQL Query receiver again
	receiver, config, consumer = createTestLogsReceiverForPostgres(t, externalPort, receiverCreateSettings)
	config.CollectionInterval = time.Second
	config.StorageID = &storageExtension.ID
	config.Queries = []sqlquery.Query{
		{
			SQL: "select * from simple_logs where id > $1",
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       "body",
					AttributeColumns: []string{"attribute"},
				},
			},
			TrackingColumn:     "id",
			TrackingStartValue: "0",
		},
	}
	err = receiver.Start(context.Background(), host)
	require.NoError(t, err)

	// Wait for some logs to come in.
	time.Sleep(5 * time.Second)

	// stop the SQL Query receiver
	err = receiver.Shutdown(context.Background())
	require.NoError(t, err)

	// Verify that no new logs came in
	require.Equal(t, 0, consumer.LogRecordCount())

	// write a number of new logs to the database
	newLogCount := 3
	insertPostgresSimpleLogs(t, dbContainer, initialLogCount, newLogCount)

	// start the SQL Query receiver again
	receiver, config, consumer = createTestLogsReceiverForPostgres(t, externalPort, receiverCreateSettings)
	config.CollectionInterval = time.Second
	config.StorageID = &storageExtension.ID
	config.Queries = []sqlquery.Query{
		{
			SQL: "select * from simple_logs where id > $1",
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       "body",
					AttributeColumns: []string{"attribute"},
				},
			},
			TrackingColumn:     "id",
			TrackingStartValue: "0",
		},
	}
	err = receiver.Start(context.Background(), host)
	require.NoError(t, err)

	// Wait for new logs to come in.
	require.Eventuallyf(
		t,
		func() bool {
			return consumer.LogRecordCount() > 0
		},
		1*time.Minute,
		1*time.Second,
		"failed to receive more than 0 logs",
	)

	// stop the SQL Query receiver
	err = receiver.Shutdown(context.Background())
	require.NoError(t, err)

	// Verify that the newly added logs were received.
	require.Equal(t, newLogCount, consumer.LogRecordCount())
	printLogs(consumer.AllLogs())
}

func startPostgresDbContainer(t *testing.T, externalPort string) testcontainers.Container {
	req := testcontainers.ContainerRequest{
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
		ExposedPorts: []string{externalPort + ":" + postgresqlPort},
		WaitingFor: wait.ForListeningPort(nat.Port(postgresqlPort)).
			WithStartupTimeout(2 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	require.NoError(t, err)
	return container
}

func createTestLogsReceiverForPostgres(t *testing.T, externalPort string, receiverCreateSettings receiver.Settings) (*logsReceiver, *Config, *consumertest.LogsSink) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.CollectionInterval = time.Second
	config.Driver = "postgres"
	config.DataSource = fmt.Sprintf("host=localhost port=%s user=otel password=otel sslmode=disable", externalPort)

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

func printLogs(allLogs []plog.Logs) {
	for logIndex := 0; logIndex < len(allLogs); logIndex++ {
		logs := allLogs[logIndex]
		for resourceIndex := 0; resourceIndex < logs.ResourceLogs().Len(); resourceIndex++ {
			resource := logs.ResourceLogs().At(resourceIndex)
			for scopeIndex := 0; scopeIndex < resource.ScopeLogs().Len(); scopeIndex++ {
				scope := resource.ScopeLogs().At(scopeIndex)
				for recordIndex := 0; recordIndex < scope.LogRecords().Len(); recordIndex++ {
					logRecord := scope.LogRecords().At(recordIndex)
					fmt.Printf("log %v resource %v scope %v log %v body: %v\n", logIndex, resourceIndex, scopeIndex, recordIndex, logRecord.Body().Str())
				}
			}
		}
	}
}

func insertPostgresSimpleLogs(t *testing.T, container testcontainers.Container, existingLogID, newLogCount int) {
	for newLogID := existingLogID + 1; newLogID <= existingLogID+newLogCount; newLogID++ {
		query := fmt.Sprintf("insert into simple_logs (id, insert_time, body, attribute) values (%d, now(), 'another log %d', 'TLSv1.2');", newLogID, newLogID)
		returnValue, returnMessageReader, err := container.Exec(context.Background(), []string{
			"psql", "-U", "otel", "-c", query,
		})
		require.NoError(t, err)
		returnMessageBuffer := new(strings.Builder)
		_, err = io.Copy(returnMessageBuffer, returnMessageReader)
		require.NoError(t, err)
		returnMessage := returnMessageBuffer.String()

		assert.Equal(t, 0, returnValue)
		assert.Contains(t, returnMessage, "INSERT 0 1")
	}
}

func TestPostgresqlIntegrationMetrics(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
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
				WaitingFor: wait.ForListeningPort(nat.Port(postgresqlPort)).
					WithStartupTimeout(2 * time.Minute),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = "postgres"
				rCfg.DataSource = fmt.Sprintf("host=%s port=%s user=otel password=otel sslmode=disable",
					ci.Host(t), ci.MappedPort(t, postgresqlPort))
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
	t.Skip("Skipping the test until https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27577 is fixed")
	if runtime.GOARCH == "arm64" {
		t.Skip("Incompatible with arm64")
	}
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration", "oracle"),
					Dockerfile: "Dockerfile.oracledb",
				},
				ExposedPorts: []string{oraclePort},
				// The Oracle DB container takes close to 10 minutes on a local machine
				// to do the default setup, so the best way to account for startup time
				// is to wait for the container to be healthy before continuing test.
				WaitingFor: wait.NewHealthStrategy().WithStartupTimeout(30 * time.Minute),
			}),
		scraperinttest.WithCreateContainerTimeout(30*time.Minute),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = "oracle"
				rCfg.DataSource = fmt.Sprintf("oracle://otel:p@ssw%%25rd@%s:%s/XE",
					ci.Host(t), ci.MappedPort(t, oraclePort))
				rCfg.Queries = []sqlquery.Query{
					{
						SQL: "select genre, count(*) as count, avg(imdb_rating) as avg from sys.movie group by genre",
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
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
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
				WaitingFor:   wait.ForListeningPort(nat.Port(mysqlPort)).WithStartupTimeout(2 * time.Minute),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = "mysql"
				rCfg.DataSource = fmt.Sprintf("otel:otel@tcp(%s:%s)/otel",
					ci.Host(t), ci.MappedPort(t, mysqlPort))
				rCfg.Queries = []sqlquery.Query{
					{
						SQL: "select genre, count(*), avg(imdb_rating) from movie group by genre",
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
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", "mysql", "expected.yaml"),
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}

func testAllSimpleLogs(t *testing.T, logs []plog.Logs) {
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
		logAttribute, _ := logRecord.Attributes().Get("attribute")
		assert.Equal(t, expectedLogAttributes[i], logAttribute.Str())
	}
}
