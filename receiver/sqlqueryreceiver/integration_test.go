// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package sqlqueryreceiver

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	postgresqlPort = "5432"
	oraclePort     = "1521"
	mysqlPort      = "3306"
)

func TestPostgresqlIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
					Dockerfile: "Dockerfile.postgresql",
				},
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
				rCfg.Queries = []Query{
					{
						SQL: "select genre, count(*), avg(imdb_rating) from movie group by genre",
						Metrics: []MetricCfg{
							{
								MetricName:       "genre.count",
								ValueColumn:      "count",
								AttributeColumns: []string{"genre"},
								ValueType:        MetricValueTypeInt,
								DataType:         MetricTypeGauge,
							},
							{
								MetricName:       "genre.imdb",
								ValueColumn:      "avg",
								AttributeColumns: []string{"genre"},
								ValueType:        MetricValueTypeDouble,
								DataType:         MetricTypeGauge,
							},
						},
					},
					{
						SQL: "select 1::smallint as a, 2::integer as b, 3::bigint as c, 4.1::decimal as d," +
							" 4.2::numeric as e, 4.3::real as f, 4.4::double precision as g, null as h",
						Metrics: []MetricCfg{
							{
								MetricName:  "a",
								ValueColumn: "a",
								ValueType:   MetricValueTypeInt,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "b",
								ValueColumn: "b",
								ValueType:   MetricValueTypeInt,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "c",
								ValueColumn: "c",
								ValueType:   MetricValueTypeInt,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "d",
								ValueColumn: "d",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "e",
								ValueColumn: "e",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "f",
								ValueColumn: "f",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "g",
								ValueColumn: "g",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "h",
								ValueColumn: "h",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
						},
					},
				}
			}),
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", "expected_postgresql.yaml"),
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}

// This test ensures the collector can connect to an Oracle DB, and properly get metrics. It's not intended to
// test the receiver itself.
func TestOracleDBIntegration(t *testing.T) {
	if runtime.GOARCH == "arm64" {
		t.Skip("Incompatible with arm64")
	}
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
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
				rCfg.DataSource = fmt.Sprintf("oracle://otel:password@%s:%s/XE",
					ci.Host(t), ci.MappedPort(t, oraclePort))
				rCfg.Queries = []Query{
					{
						SQL: "select genre, count(*) as count, avg(imdb_rating) as avg from sys.movie group by genre",
						Metrics: []MetricCfg{
							{
								MetricName:       "genre.count",
								ValueColumn:      "COUNT",
								AttributeColumns: []string{"GENRE"},
								ValueType:        MetricValueTypeInt,
								DataType:         MetricTypeGauge,
							},
							{
								MetricName:       "genre.imdb",
								ValueColumn:      "AVG",
								AttributeColumns: []string{"GENRE"},
								ValueType:        MetricValueTypeDouble,
								DataType:         MetricTypeGauge,
							},
						},
					},
				}
			}),
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", "expected_oracledb.yaml"),
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}

func TestMysqlIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
					Dockerfile: "Dockerfile.mysql",
				},
				ExposedPorts: []string{mysqlPort},
				WaitingFor:   wait.ForListeningPort(nat.Port(mysqlPort)).WithStartupTimeout(2 * time.Minute),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Driver = "mysql"
				rCfg.DataSource = fmt.Sprintf("otel:otel@tcp(%s:%s)/otel",
					ci.Host(t), ci.MappedPort(t, mysqlPort))
				rCfg.Queries = []Query{
					{
						SQL: "select genre, count(*), avg(imdb_rating) from movie group by genre",
						Metrics: []MetricCfg{
							{
								MetricName:       "genre.count",
								ValueColumn:      "count(*)",
								AttributeColumns: []string{"genre"},
								ValueType:        MetricValueTypeInt,
								DataType:         MetricTypeGauge,
							},
							{
								MetricName:       "genre.imdb",
								ValueColumn:      "avg(imdb_rating)",
								AttributeColumns: []string{"genre"},
								ValueType:        MetricValueTypeDouble,
								DataType:         MetricTypeGauge,
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
						Metrics: []MetricCfg{
							{
								MetricName:  "a",
								ValueColumn: "a",
								ValueType:   MetricValueTypeInt,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "b",
								ValueColumn: "b",
								ValueType:   MetricValueTypeInt,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "c",
								ValueColumn: "c",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "d",
								ValueColumn: "d",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "e",
								ValueColumn: "e",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
							{
								MetricName:  "f",
								ValueColumn: "f",
								ValueType:   MetricValueTypeDouble,
								DataType:    MetricTypeGauge,
							},
						},
					},
				}
			}),
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", "expected_mysql.yaml"),
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
