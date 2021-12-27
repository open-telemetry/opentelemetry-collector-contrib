// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package mongodbreceiver

import (
	"context"
	"net"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/metadata"
)

func TestMongoDBIntegration(t *testing.T) {
	container := getContainer(t, containerRequest4_0)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()
	hostname, err := container.Host(context.Background())
	require.NoError(t, err)

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddr{{
		net.JoinHostPort(hostname, "37017"),
	}}
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.Insecure = true

	consumer := new(consumertest.MetricsSink)
	settings := componenttest.NewNopReceiverCreateSettings()
	rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")

	md := consumer.AllMetrics()[0]
	require.Equal(t, 1, md.ResourceMetrics().Len())
	ilms := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilms.Len())
	metrics := ilms.At(0).Metrics()
	require.Equal(t, 12, metrics.Len())
	require.NoError(t, rcvr.Shutdown(context.Background()))

	validateResult(t, metrics)
}

var (
	containerRequest4_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join(".", "testdata"),
			Dockerfile: "Dockerfile.mongodb",
		},
		ExposedPorts: []string{"37017:27017"},
		WaitingFor: wait.ForListeningPort("27017").
			WithStartupTimeout(2 * time.Minute),
	}
)

func getContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)
	return container
}

func validateResult(t *testing.T, metrics pdata.MetricSlice) {
	require.Equal(t, len(metadata.M.Names()), metrics.Len())
	exists := make(map[string]bool)

	unenumAttributeSet := []string{
		metadata.A.Database,
	}

	enumAttributeSet := []string{
		metadata.A.MemoryType,
		metadata.A.Operation,
		metadata.A.ConnectionType,
		metadata.A.Type,
	}

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		require.Contains(t, metadata.M.Names(), m.Name())

		metricIntr := metadata.M.ByName(m.Name())
		require.Equal(t, metricIntr.New().DataType(), m.DataType())
		var dps pdata.NumberDataPointSlice
		switch m.DataType() {
		case pdata.MetricDataTypeGauge:
			dps = m.Gauge().DataPoints()
		case pdata.MetricDataTypeSum:
			dps = m.Sum().DataPoints()
		}

		for j := 0; j < dps.Len(); j++ {
			key := m.Name()
			dp := dps.At(j)

			for _, attribute := range unenumAttributeSet {
				_, ok := dp.Attributes().Get(attribute)
				if ok {
					key = key + " " + attribute
				}
			}

			for _, attribute := range enumAttributeSet {
				attributeVal, ok := dp.Attributes().Get(attribute)
				if ok {
					key += " " + attributeVal.AsString()
				}
			}
			exists[key] = true
		}
	}

	require.Equal(t, map[string]bool{
		"mongodb.cache.operations hits":                   true,
		"mongodb.cache.operations misses":                 true,
		"mongodb.collections database":                    true,
		"mongodb.data.size database":                      true,
		"mongodb.connections database active":             true,
		"mongodb.connections database available":          true,
		"mongodb.connections database current":            true,
		"mongodb.extents database":                        true,
		"mongodb.global_lock.hold":                        true,
		"mongodb.index.size database":                     true,
		"mongodb.index.count database":                    true,
		"mongodb.memory.usage database mapped":            true,
		"mongodb.memory.usage database mappedWithJournal": true,
		"mongodb.memory.usage database resident":          true,
		"mongodb.memory.usage database virtual":           true,
		"mongodb.objects database":                        true,
		"mongodb.operations command":                      true,
		"mongodb.operations delete":                       true,
		"mongodb.operations getmore":                      true,
		"mongodb.operations insert":                       true,
		"mongodb.operations query":                        true,
		"mongodb.operations update":                       true,
		"mongodb.storage.size database":                   true,
	}, exists)
}
