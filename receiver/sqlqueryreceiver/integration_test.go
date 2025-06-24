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

	_ "github.com/SAP/go-hdb/driver" // register Db driver
	"github.com/docker/go-connections/nat"
	_ "github.com/go-sql-driver/mysql"                      // register Db driver
	_ "github.com/lib/pq"                                   // register Db driver
	_ "github.com/microsoft/go-mssqldb"                     // register Db driver
	_ "github.com/microsoft/go-mssqldb/integratedauth/krb5" // register Db driver
	_ "github.com/sijms/go-ora/v2"                          // register Db driver
	_ "github.com/snowflakedb/gosnowflake"                  // register Db driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	_ "github.com/thda/tds" // register Db driver
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal/metadata"
)

const (
	postgresqlPort = "5432"
	oraclePort     = "1521"
	mysqlPort      = "3306"
	sqlServerPort  = "1433"
	sapAsePort     = "5000"
)

type dbEngineUnderTest struct {
	Port               string
	SQLParameter       func(position int) string
	CheckCompatibility func(t *testing.T)
	ConnectionString   func(host string, externalPort nat.Port) string
	Driver             string
	ConvertColumnName  func(string) string
	ContainerRequest   testcontainers.ContainerRequest
}

var (
	Postgres = dbEngineUnderTest{
		Port: postgresqlPort,
		SQLParameter: func(position int) string {
			return fmt.Sprintf("$%d", position)
		},
		CheckCompatibility: func(_ *testing.T) {
			// No compatibility checks needed for Postgres
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("host=%s port=%s user=otel password=otel sslmode=disable", host, externalPort.Port())
		},
		Driver:            "postgres",
		ConvertColumnName: func(name string) string { return name },
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
	MySQL = dbEngineUnderTest{
		Port: mysqlPort,
		SQLParameter: func(_ int) string {
			return "?"
		},
		CheckCompatibility: func(_ *testing.T) {
			// No compatibility checks needed for MySQL
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("otel:otel@tcp(%s:%s)/otel", host, externalPort.Port())
		},
		Driver:            "mysql",
		ConvertColumnName: func(name string) string { return name },
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
	Oracle = dbEngineUnderTest{
		Port: oraclePort,
		SQLParameter: func(position int) string {
			return fmt.Sprintf(":%d", position)
		},
		CheckCompatibility: func(t *testing.T) {
			t.Skip("Skipping the test until https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27577 is fixed")
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("oracle://otel:otel@%s:%s/FREEPDB1", host, externalPort.Port())
		},
		Driver:            "oracle",
		ConvertColumnName: strings.ToUpper,
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "gvenzl/oracle-free:slim-faststart",
			Env: map[string]string{
				"ORACLE_PASSWORD":   "mysecurepassword",
				"APP_USER":          "otel",
				"APP_USER_PASSWORD": "otel",
			},
			Files: []testcontainers.ContainerFile{{
				HostFilePath:      filepath.Join("testdata", "integration", "oracle", "init.sql"),
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
				FileMode:          700,
			}},
			ExposedPorts: []string{oraclePort},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort(oraclePort),
				wait.ForLog("DATABASE IS READY TO USE!"),
			).WithDeadline(5 * time.Minute),
		},
	}
	SQLServer = dbEngineUnderTest{
		Port: sqlServerPort,
		SQLParameter: func(position int) string {
			return fmt.Sprintf("@p%d", position)
		},
		CheckCompatibility: func(t *testing.T) {
			if runtime.GOARCH == "arm64" {
				t.Skip("Incompatible with arm64")
			}
			t.Skip("Skipping the test until https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27577 is fixed")
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("sqlserver://otel:YourStrong%%21Passw0rd@%s:%s?database=otel", host, externalPort.Port())
		},
		Driver:            "sqlserver",
		ConvertColumnName: func(name string) string { return name },
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration", "sqlserver"),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{sqlServerPort},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort(sqlServerPort),
				wait.ForLog("Initiation of otel database is complete"),
			).WithDeadline(5 * time.Minute),
		},
	}
	SapASE = dbEngineUnderTest{
		Port: sapAsePort,
		SQLParameter: func(_ int) string {
			return "?"
		},
		CheckCompatibility: func(t *testing.T) {
			t.Skip("Skipping the test until https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27577 is fixed")
		},
		ConnectionString: func(host string, externalPort nat.Port) string {
			return fmt.Sprintf("tds://otel:otel1234@%s:%s/otel", host, externalPort.Port())
		},
		Driver:            "tds",
		ConvertColumnName: func(name string) string { return name },
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "datagrip/sybase:16.0",
			Env: map[string]string{
				"SYBASE_USER":     "otel",
				"SYBASE_DB":       "otel",
				"SYBASE_PASSWORD": "otel1234",
			},
			Files: []testcontainers.ContainerFile{{
				HostFilePath:      filepath.Join("testdata", "integration", "sybase", "entrypoint.sh"),
				ContainerFilePath: "/entrypoint.sh",
				FileMode:          777,
			}},
			ExposedPorts: []string{sapAsePort},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort(sapAsePort).WithStartupTimeout(5*time.Minute),
				wait.ForLog("SYBASE INITIALIZED").WithStartupTimeout(5*time.Minute),
			).WithDeadline(5 * time.Minute),
		},
	}
)

func TestIntegrationLogsTracking(t *testing.T) {
	testGroupedByDbEngine := map[string][]struct {
		name    string
		runTest func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container)
	}{
		Postgres.Driver: {
			{name: "PostgresWithStorage", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithStorage(t, engine, container, "select * from simple_logs where id > $1 order by id asc")
			}},
			{name: "PostgresById", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "id", "0", "select * from simple_logs where id > $1 order by id asc")
			}},
			{name: "PostgresByTimestamp", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "insert_time", "2022-06-03 21:00:00+00", "select * from simple_logs where insert_time > $1 order by insert_time asc")
			}},
		},
		MySQL.Driver: {
			{name: "MySQLWithStorage", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithStorage(t, engine, container, "select * from simple_logs where id > ? order by id asc")
			}},
			{name: "MySQLById", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "id", "0", "select * from simple_logs where id > ? order by id asc")
			}},
			{name: "MySQLByTimestamp", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "insert_time", "2022-06-03 21:00:00", "select * from simple_logs where insert_time > ? order by insert_time asc")
			}},
		},
		SQLServer.Driver: {
			{name: "SQLServerWithStorage", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithStorage(t, engine, container, "select * from simple_logs where id > @p1 order by id asc")
			}},
			{name: "SQLServerById", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "id", "0", "select * from simple_logs where id > @p1 order by id asc")
			}},
			{name: "SQLServerByTimestamp", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "insert_time", "2022-06-03 21:00:00", "select * from simple_logs where insert_time > @p1 order by insert_time asc")
			}},
		},
		Oracle.Driver: {
			{name: "OracleWithStorage", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithStorage(t, engine, container, "select * from simple_logs where ID > :1 order by ID asc")
			}},
			{name: "OracleById", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "ID", "0", "select * from simple_logs where ID > :1 order by ID asc")
			}},
			{name: "OracleByTimestamp", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "INSERT_TIME", "2022-06-03T21:00:00.000Z", "select * from simple_logs where INSERT_TIME > TO_TIMESTAMP_TZ(:1, 'YYYY-MM-DD\"T\"HH24:MI:SS.FF6TZH:TZM') order by INSERT_TIME asc")
			}},
		},
		SapASE.Driver: {
			{name: "SapASEWithStorage", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithStorage(t, engine, container, "select * from simple_logs where convert(varchar,id)  > ? order by id asc")
			}},
			{name: "SapASEById", runTest: func(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) {
				runTestForLogTrackingWithoutStorage(t, engine, container, "id", "0", "select * from simple_logs where convert(varchar,id)  > ? order by id asc")
			}},
		},
	}

	for driver, dbEngineTests := range testGroupedByDbEngine {
		t.Run(driver, func(t *testing.T) {
			engine := getDbEngine(driver)
			engine.CheckCompatibility(t)
			dbContainer, err := testcontainers.GenericContainer(
				context.Background(),
				testcontainers.GenericContainerRequest{
					ContainerRequest: engine.ContainerRequest,
					Started:          true,
				},
			)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, dbContainer.Terminate(context.Background()))
			}()

			for _, test := range dbEngineTests {
				t.Run(test.name, func(t *testing.T) {
					test.runTest(t, engine, dbContainer)
				})
			}
		})
	}
}

func getDbEngine(driver string) dbEngineUnderTest {
	switch driver {
	case Postgres.Driver:
		return Postgres
	case MySQL.Driver:
		return MySQL
	case SQLServer.Driver:
		return SQLServer
	case Oracle.Driver:
		return Oracle
	case SapASE.Driver:
		return SapASE
	default:
		panic(fmt.Sprintf("unsupported driver: %s", driver))
	}
}

func runTestForLogTrackingWithStorage(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container, querySQL string) {
	dbHost, dbPort := getContainerHostAndPort(t, container, engine.Port)
	storageDir := t.TempDir()
	storageExtension := storagetest.NewFileBackedStorageExtension("test", storageDir)

	trackingColumn := engine.ConvertColumnName("id")
	trackingStartValue := "0"

	receiverCreateSettings := receivertest.NewNopSettings(metadata.Type)
	receiver, config, consumer := createTestLogsReceiver(t, engine.Driver, engine.ConnectionString(dbHost, dbPort), receiverCreateSettings)
	config.CollectionInterval = time.Second
	config.Telemetry.Logs.Query = true
	config.StorageID = &storageExtension.ID
	config.Queries = []sqlquery.Query{
		{
			SQL: querySQL,
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       engine.ConvertColumnName("body"),
					AttributeColumns: []string{engine.ConvertColumnName("attribute")},
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
	testAllSimpleLogs(t, consumer.AllLogs(), engine.ConvertColumnName("attribute"))

	receiver, config, consumer = createTestLogsReceiver(t, engine.Driver, engine.ConnectionString(dbHost, dbPort), receiverCreateSettings)
	config.CollectionInterval = time.Second
	config.Telemetry.Logs.Query = true
	config.StorageID = &storageExtension.ID
	config.Queries = []sqlquery.Query{
		{
			SQL: querySQL,
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       engine.ConvertColumnName("body"),
					AttributeColumns: []string{engine.ConvertColumnName("attribute")},
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
	insertSimpleLogs(t, engine, container, initialLogCount, newLogCount)
	defer cleanupSimpleLogs(t, engine, container, initialLogCount)

	receiver, config, consumer = createTestLogsReceiver(t, engine.Driver, engine.ConnectionString(dbHost, dbPort), receiverCreateSettings)
	config.CollectionInterval = time.Second
	config.Telemetry.Logs.Query = true
	config.StorageID = &storageExtension.ID
	config.Queries = []sqlquery.Query{
		{
			SQL: querySQL,
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       engine.ConvertColumnName("body"),
					AttributeColumns: []string{engine.ConvertColumnName("attribute")},
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
}

func runTestForLogTrackingWithoutStorage(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container, trackingColumn, trackingStartValue, sqlQuery string) {
	receiverCreateSettings := receivertest.NewNopSettings(metadata.Type)
	dbHost, dbPort := getContainerHostAndPort(t, container, engine.Port)
	receiver, config, consumer := createTestLogsReceiver(t, engine.Driver, engine.ConnectionString(dbHost, dbPort), receiverCreateSettings)
	config.CollectionInterval = 100 * time.Millisecond
	config.Telemetry.Logs.Query = true

	trackingColumn = engine.ConvertColumnName(trackingColumn)

	config.Queries = []sqlquery.Query{
		{
			SQL: sqlQuery,
			Logs: []sqlquery.LogsCfg{
				{
					BodyColumn:       engine.ConvertColumnName("body"),
					AttributeColumns: []string{engine.ConvertColumnName("attribute")},
				},
			},
			TrackingColumn:     trackingColumn,
			TrackingStartValue: trackingStartValue,
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
	testAllSimpleLogs(t, consumer.AllLogs(), engine.ConvertColumnName("attribute"))

	err = receiver.Shutdown(context.Background())
	require.NoError(t, err)
}

func getContainerHostAndPort(t *testing.T, container testcontainers.Container, port string) (string, nat.Port) {
	dbPort, err := container.MappedPort(context.Background(), nat.Port(port))
	require.NoError(t, err)
	dbHost, err := container.Host(context.Background())
	require.NoError(t, err)
	return dbHost, dbPort
}

func insertSimpleLogs(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container, existingLogID, newLogCount int) {
	db := openDatabase(t, engine, container)
	defer db.Close()

	stmt := prepareStatement(t, db, fmt.Sprintf("INSERT INTO simple_logs (id, body, attribute) VALUES (%s, %s, %s)", engine.SQLParameter(1), engine.SQLParameter(2), engine.SQLParameter(3)))
	defer stmt.Close()

	for newLogID := existingLogID + 1; newLogID <= existingLogID+newLogCount; newLogID++ {
		_, err := stmt.Exec(newLogID, fmt.Sprintf("another log %d", newLogID), "TLSv1.2")
		require.NoError(t, err)
	}
}

func cleanupSimpleLogs(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container, existingLogID int) {
	db := openDatabase(t, engine, container)
	defer db.Close()

	deleteStatement := "DELETE FROM simple_logs WHERE id > " + engine.SQLParameter(1)
	stmt := prepareStatement(t, db, deleteStatement)
	defer stmt.Close()

	_, err := stmt.Exec(existingLogID)
	require.NoError(t, err)
}

func openDatabase(t *testing.T, engine dbEngineUnderTest, container testcontainers.Container) *sql.DB {
	externalPort, err := container.MappedPort(context.Background(), nat.Port(engine.Port))
	require.NoError(t, err)

	host, err := container.Host(context.Background())
	require.NoError(t, err)

	db, err := sql.Open(engine.Driver, engine.ConnectionString(host, externalPort))
	require.NoError(t, err)
	return db
}

func prepareStatement(t *testing.T, db *sql.DB, query string) *sql.Stmt {
	stmt, err := db.Prepare(query)
	require.NoError(t, err)
	return stmt
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
	MySQL.CheckCompatibility(t)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(MySQL.ContainerRequest),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = MySQL.Driver
				rCfg.DataSource = MySQL.ConnectionString(ci.Host(t), nat.Port(ci.MappedPort(t, MySQL.Port)))
				rCfg.MaxOpenConn = 5
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

func TestSQLServerIntegrationMetrics(t *testing.T) {
	SQLServer.CheckCompatibility(t)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(SQLServer.ContainerRequest),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = SQLServer.Driver
				rCfg.DataSource = SQLServer.ConnectionString(ci.Host(t), nat.Port(ci.MappedPort(t, SQLServer.Port)))
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

func TestSapASEIntegrationMetrics(t *testing.T) {
	SapASE.CheckCompatibility(t)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(SapASE.ContainerRequest),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = SapASE.Driver
				rCfg.DataSource = SapASE.ConnectionString(ci.Host(t), nat.Port(ci.MappedPort(t, SapASE.Port)))
				rCfg.Queries = []sqlquery.Query{
					{
						SQL: "SELECT genre, COUNT(*) AS movie_count, AVG(imdb_rating) AS movie_avg FROM movie GROUP BY genre ORDER BY genre",
						Metrics: []sqlquery.MetricCfg{
							{
								MetricName:       "genre.count",
								ValueColumn:      "movie_count",
								AttributeColumns: []string{"genre"},
								ValueType:        sqlquery.MetricValueTypeInt,
								DataType:         sqlquery.MetricTypeGauge,
							},
							{
								MetricName:       "genre.imdb",
								ValueColumn:      "movie_avg",
								AttributeColumns: []string{"genre"},
								ValueType:        sqlquery.MetricValueTypeDouble,
								DataType:         sqlquery.MetricTypeGauge,
							},
						},
					},
				}
			}),
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", "sybase", "expected.yaml"),
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreMetricsOrder(),
		),
	).Run(t)
}
