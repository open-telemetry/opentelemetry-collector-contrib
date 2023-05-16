// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration

package sqlqueryreceiver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestPostgresIntegration(t *testing.T) {
	externalPort := "15432"
	internalPort := "5432"
	waitStrategy := wait.ForListeningPort(nat.Port(internalPort)).WithStartupTimeout(2 * time.Minute)
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.postgresql",
		},
		ExposedPorts: []string{externalPort + ":" + internalPort},
		WaitingFor:   waitStrategy,
	}
	ctx := context.Background()

	_, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	require.NoError(t, err)

	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Driver = "postgres"
	config.DataSource = fmt.Sprintf("host=localhost port=%s user=otel password=otel sslmode=disable", externalPort)
	genreKey := "genre"
	config.Queries = []Query{
		{
			SQL: "select genre, count(*), avg(imdb_rating) from movie group by genre",
			Metrics: []MetricCfg{
				{
					MetricName:       "genre.count",
					ValueColumn:      "count",
					AttributeColumns: []string{genreKey},
					ValueType:        MetricValueTypeInt,
					DataType:         MetricTypeGauge,
				},
				{
					MetricName:       "genre.imdb",
					ValueColumn:      "avg",
					AttributeColumns: []string{genreKey},
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
	consumer := &consumertest.MetricsSink{}
	receiver, err := factory.CreateMetricsReceiver(
		ctx,
		receivertest.NewNopCreateSettings(),
		config,
		consumer,
	)
	require.NoError(t, err)
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	require.Eventuallyf(
		t,
		func() bool {
			return consumer.DataPointCount() > 0
		},
		2*time.Minute,
		1*time.Second,
		"failed to receive more than 0 metrics",
	)
	metrics := consumer.AllMetrics()[0]
	rms := metrics.ResourceMetrics()
	testMovieMetrics(t, rms.At(0), genreKey)
	testPGTypeMetrics(t, rms.At(1))
}

// This test ensures the collector can connect to an Oracle DB, and properly get metrics. It's not intended to
// test the receiver itself.
func TestOracleDBIntegration(t *testing.T) {
	externalPort := "51521"
	internalPort := "1521"

	// The Oracle DB container takes close to 10 minutes on a local machine to do the default setup, so the best way to
	// account for startup time is to wait for the container to be healthy before continuing test.
	waitStrategy := wait.NewHealthStrategy().WithStartupTimeout(15 * time.Minute)
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.oracledb",
		},
		ExposedPorts: []string{externalPort + ":" + internalPort},
		WaitingFor:   waitStrategy,
	}
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	require.NotNil(t, container)
	require.NoError(t, err)

	genreKey := "GENRE"
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Driver = "oracle"
	config.DataSource = "oracle://otel:password@localhost:51521/XE"
	config.Queries = []Query{
		{
			SQL: "select genre, count(*) as count, avg(imdb_rating) as avg from sys.movie group by genre",
			Metrics: []MetricCfg{
				{
					MetricName:       "genre.count",
					ValueColumn:      "COUNT",
					AttributeColumns: []string{genreKey},
					ValueType:        MetricValueTypeInt,
					DataType:         MetricTypeGauge,
				},
				{
					MetricName:       "genre.imdb",
					ValueColumn:      "AVG",
					AttributeColumns: []string{genreKey},
					ValueType:        MetricValueTypeDouble,
					DataType:         MetricTypeGauge,
				},
			},
		},
	}
	consumer := &consumertest.MetricsSink{}
	receiver, err := factory.CreateMetricsReceiver(
		ctx,
		receivertest.NewNopCreateSettings(),
		config,
		consumer,
	)
	require.NoError(t, err)
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	require.Eventuallyf(
		t,
		func() bool {
			return consumer.DataPointCount() > 0
		},
		15*time.Minute,
		1*time.Second,
		"failed to receive more than 0 metrics",
	)
	metrics := consumer.AllMetrics()[0]
	rms := metrics.ResourceMetrics()
	testMovieMetrics(t, rms.At(0), genreKey)
}

func TestMysqlIntegration(t *testing.T) {
	externalPort := "13306"
	internalPort := "3306"
	waitStrategy := wait.ForListeningPort(nat.Port(internalPort)).WithStartupTimeout(2 * time.Minute)
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mysql",
		},
		ExposedPorts: []string{externalPort + ":" + internalPort},
		WaitingFor:   waitStrategy,
	}
	ctx := context.Background()

	_, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	require.NoError(t, err)

	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Driver = "mysql"
	config.DataSource = fmt.Sprintf("otel:otel@tcp(localhost:%s)/otel", externalPort)
	genreKey := "genre"
	config.Queries = []Query{
		{
			SQL: "select genre, count(*), avg(imdb_rating) from movie group by genre",
			Metrics: []MetricCfg{
				{
					MetricName:       "genre.count",
					ValueColumn:      "count(*)",
					AttributeColumns: []string{genreKey},
					ValueType:        MetricValueTypeInt,
					DataType:         MetricTypeGauge,
				},
				{
					MetricName:       "genre.imdb",
					ValueColumn:      "avg(imdb_rating)",
					AttributeColumns: []string{genreKey},
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
	consumer := &consumertest.MetricsSink{}
	receiver, err := factory.CreateMetricsReceiver(
		ctx,
		receivertest.NewNopCreateSettings(),
		config,
		consumer,
	)
	require.NoError(t, err)
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	require.Eventuallyf(
		t,
		func() bool {
			return consumer.DataPointCount() > 0
		},
		2*time.Minute,
		1*time.Second,
		"failed to receive more than 0 metrics",
	)
	metrics := consumer.AllMetrics()[0]
	rms := metrics.ResourceMetrics()
	testMovieMetrics(t, rms.At(0), genreKey)
	testMysqlTypeMetrics(t, rms.At(1))
}

func testMovieMetrics(t *testing.T, rm pmetric.ResourceMetrics, genreAttrKey string) {
	sms := rm.ScopeMetrics()
	assert.Equal(t, 1, sms.Len())
	sm := sms.At(0)
	ms := sm.Metrics()
	assert.Equal(t, 4, ms.Len())

	metricsByName := map[string][]pmetric.Metric{}
	for i := 0; i < ms.Len(); i++ {
		metric := ms.At(i)
		name := metric.Name()
		metricsByName[name] = append(metricsByName[name], metric)
	}

	for _, metric := range metricsByName["genre.count"] {
		pt := metric.Gauge().DataPoints().At(0)
		genre, _ := pt.Attributes().Get(genreAttrKey)
		genreStr := genre.AsString()
		switch genreStr {
		case "SciFi":
			assert.EqualValues(t, 3, pt.IntValue())
		case "Action":
			assert.EqualValues(t, 2, pt.IntValue())
		default:
			assert.Failf(t, "unexpected genre: %s", genreStr)
		}
	}

	for _, metric := range metricsByName["genre.imdb"] {
		pt := metric.Gauge().DataPoints().At(0)
		genre, _ := pt.Attributes().Get(genreAttrKey)
		genreStr := genre.AsString()
		switch genreStr {
		case "SciFi":
			assert.InDelta(t, 8.2, pt.DoubleValue(), 0.1)
		case "Action":
			assert.InDelta(t, 7.65, pt.DoubleValue(), 0.1)
		default:
			assert.Failf(t, "unexpected genre: %s", genreStr)
		}
	}
}

func testPGTypeMetrics(t *testing.T, rm pmetric.ResourceMetrics) {
	sms := rm.ScopeMetrics()
	assert.Equal(t, 1, sms.Len())
	sm := sms.At(0)
	ms := sm.Metrics()
	for i := 0; i < ms.Len(); i++ {
		metric := ms.At(0)
		switch metric.Name() {
		case "a":
			assertIntGaugeEquals(t, 1, metric)
		case "b":
			assertIntGaugeEquals(t, 2, metric)
		case "c":
			assertIntGaugeEquals(t, 3, metric)
		case "d":
			assertDoubleGaugeEquals(t, 4.1, metric)
		case "e":
			assertDoubleGaugeEquals(t, 4.2, metric)
		case "f":
			assertDoubleGaugeEquals(t, 4.3, metric)
		case "g":
			assertDoubleGaugeEquals(t, 4.4, metric)
		}
	}
}

func testMysqlTypeMetrics(t *testing.T, rm pmetric.ResourceMetrics) {
	sms := rm.ScopeMetrics()
	assert.Equal(t, 1, sms.Len())
	sm := sms.At(0)
	ms := sm.Metrics()
	for i := 0; i < ms.Len(); i++ {
		metric := ms.At(0)
		switch metric.Name() {
		case "a":
			assertIntGaugeEquals(t, 1, metric)
		case "b":
			assertIntGaugeEquals(t, 2, metric)
		case "c":
			assertDoubleGaugeEquals(t, 3.1, metric)
		case "d":
			assertDoubleGaugeEquals(t, 3.2, metric)
		case "e":
			assertDoubleGaugeEquals(t, 3.3, metric)
		case "f":
			assertDoubleGaugeEquals(t, 3.4, metric)
		}
	}
}

func assertIntGaugeEquals(t *testing.T, expected int, metric pmetric.Metric) {
	assert.EqualValues(t, expected, metric.Gauge().DataPoints().At(0).IntValue())
}

func assertDoubleGaugeEquals(t *testing.T, expected float64, metric pmetric.Metric) {
	assert.InDelta(t, expected, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.1)
}
