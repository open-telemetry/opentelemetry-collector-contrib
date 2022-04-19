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
	config.DataSource = "host=localhost port=" + externalPort + " user=otel password=otel sslmode=disable"
	config.Queries = []Query{{
		SQL: "select count(*) as count, genre from movie group by genre order by genre",
		Metrics: []MetricCfg{{
			MetricName:       "movie.genres",
			ValueColumn:      "count",
			AttributeColumns: []string{"genre"},
			ValueType:        MetricValueTypeInt,
			DataType:         MetricDataTypeGauge,
		}},
	}}
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
	assert.Equal(t, 1, rms.Len())
	rm := rms.At(0)
	sms := rm.ScopeMetrics()
	assert.Equal(t, 1, sms.Len())
	sm := sms.At(0)
	ms := sm.Metrics()
	assert.Equal(t, 2, ms.Len())

	countByGenre := map[string]int64{}
	extractGenre(t, ms.At(0), countByGenre)
	extractGenre(t, ms.At(1), countByGenre)
	assert.Equal(t, map[string]int64{"Action": 2, "SciFi": 3}, countByGenre)
}

func extractGenre(t *testing.T, metric pmetric.Metric, countByGenre map[string]int64) {
	assert.Equal(t, "movie.genres", metric.Name())
	pts := metric.Gauge().DataPoints()
	assert.Equal(t, 1, pts.Len())
	pt := pts.At(0)
	attrs := pt.Attributes()
	genre, _ := attrs.Get("genre")
	countByGenre[genre.AsString()] = pt.IntVal()
}
