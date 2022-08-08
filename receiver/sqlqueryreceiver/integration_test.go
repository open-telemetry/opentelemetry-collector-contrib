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
// +build integration

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
	config.Queries = []Query{
		{
			SQL: "select genre, count(*), avg(imdb_rating) from movie group by genre",
			Metrics: []MetricCfg{
				{
					MetricName:       "genre.count",
					ValueColumn:      "count",
					AttributeColumns: []string{"genre"},
					ValueType:        MetricValueTypeInt,
					DataType:         MetricDataTypeGauge,
				},
				{
					MetricName:       "genre.imdb",
					ValueColumn:      "avg",
					AttributeColumns: []string{"genre"},
					ValueType:        MetricValueTypeDouble,
					DataType:         MetricDataTypeGauge,
				},
			},
		},
		{
			SQL: "select 1::smallint as a, 2::integer as b, 3::bigint as c, 4.1::decimal as d," +
				" 4.2::numeric as e, 4.3::real as f, 4.4::double precision as g",
			Metrics: []MetricCfg{
				{
					MetricName:  "a",
					ValueColumn: "a",
					ValueType:   MetricValueTypeInt,
					DataType:    MetricDataTypeGauge,
				},
				{
					MetricName:  "b",
					ValueColumn: "b",
					ValueType:   MetricValueTypeInt,
					DataType:    MetricDataTypeGauge,
				},
				{
					MetricName:  "c",
					ValueColumn: "c",
					ValueType:   MetricValueTypeInt,
					DataType:    MetricDataTypeGauge,
				},
				{
					MetricName:  "d",
					ValueColumn: "d",
					ValueType:   MetricValueTypeDouble,
					DataType:    MetricDataTypeGauge,
				},
				{
					MetricName:  "e",
					ValueColumn: "e",
					ValueType:   MetricValueTypeDouble,
					DataType:    MetricDataTypeGauge,
				},
				{
					MetricName:  "f",
					ValueColumn: "f",
					ValueType:   MetricValueTypeDouble,
					DataType:    MetricDataTypeGauge,
				},
				{
					MetricName:  "g",
					ValueColumn: "g",
					ValueType:   MetricValueTypeDouble,
					DataType:    MetricDataTypeGauge,
				},
			},
		},
	}
	consumer := &consumertest.MetricsSink{}
	receiver, err := factory.CreateMetricsReceiver(
		ctx,
		componenttest.NewNopReceiverCreateSettings(),
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
	testMovieMetrics(t, rms.At(0))
	testPGTypeMetrics(t, rms.At(1))
}

func testMovieMetrics(t *testing.T, rm pmetric.ResourceMetrics) {
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
		genre, _ := pt.Attributes().Get("genre")
		genreStr := genre.AsString()
		switch genreStr {
		case "SciFi":
			assert.EqualValues(t, 3, pt.IntVal())
		case "Action":
			assert.EqualValues(t, 2, pt.IntVal())
		default:
			assert.Failf(t, "unexpected genre: %s", genreStr)
		}
	}

	for _, metric := range metricsByName["genre.imdb"] {
		pt := metric.Gauge().DataPoints().At(0)
		genre, _ := pt.Attributes().Get("genre")
		genreStr := genre.AsString()
		switch genreStr {
		case "SciFi":
			assert.InDelta(t, 8.2, pt.DoubleVal(), 0.1)
		case "Action":
			assert.InDelta(t, 7.65, pt.DoubleVal(), 0.1)
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

func assertIntGaugeEquals(t *testing.T, expected int, metric pmetric.Metric) bool {
	return assert.EqualValues(t, expected, metric.Gauge().DataPoints().At(0).IntVal())
}

func assertDoubleGaugeEquals(t *testing.T, expected float64, metric pmetric.Metric) bool {
	return assert.InDelta(t, expected, metric.Gauge().DataPoints().At(0).DoubleVal(), 0.1)
}
