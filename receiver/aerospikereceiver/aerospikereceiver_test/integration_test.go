// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package aerospikereceiver_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v5"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/containertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"
)

func populateMetrics(t *testing.T, addr string, port int) {
	client, err := as.NewClient(addr, port)
	require.NoError(t, err, "failed to create client")

	// write 100 records to get some memory usage
	for i := 0; i < 100; i++ {
		key, err := as.NewKey("test", "demo", i)
		require.NoError(t, err, "failed to create key")

		bins := as.BinMap{
			"bin1": i
		}

		br := client.Put(nil, key, bins)
	}

	// perform a basic primary index query
	queryPolicy := as.NewQueryPolicy()
	statement := as.NewStatement("test", "demo", "bin1")
	_, err := c.Query(queryPolicy, statement)
	require.NoError(t, err, "failed to execute basic PI query")

	// aggregation query on primary index
	queryPolicy := as.NewQueryPolicy()
	statement := as.NewStatement("test", "demo", "bin1")
	_, err = c.QueryAggregate(queryPolicy, statement, "test_funcs", "func1") // TODO make the test UDFs
	require.NoError(t, err, "failed to execute aggregation PI query")

	// ops query on primary index
	queryPolicy := as.NewQueryPolicy()
	var write_Policy *as.WritePolicy = nil
	statement := as.NewStatement("test", "demo", "bin1")
	ops := as.GetOp() // TODO add more op types
	_, err = c.QueryExecute(queryPolicy, write_Policy, statement, ops)
	require.NoError(t, err, "failed to execute aggregation PI query")

	// perform a basic short primary index query
	queryPolicy := as.NewQueryPolicy()
	queryPolicy.ShortQuery = true
	statement := as.NewStatement("test", "demo", "bin1")
	_, err := c.Query(queryPolicy, statement)
	require.NoError(t, err, "failed to execute basic PI query")

}

func TestAerospikeIntegration(t *testing.T) {
	t.Parallel()

	ct := containertest.New(t)
	container := ct.StartImage("aerospike:ce-5.7.0.17", containertest.WithPortReady(3000))

	f := aerospikereceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*aerospikereceiver.Config)
	cfg.Endpoint = container.AddrForPort(3000)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond

	consumer := new(consumertest.MetricsSink)
	settings := componenttest.NewNopReceiverCreateSettings()
	receiver, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	time.Sleep(time.Second / 2)
	require.NoError(t, receiver.Start(context.Background(), componenttest.NewNopHost()), "failed starting metrics receiver")

	require.Eventually(t, func() bool {
		return consumer.DataPointCount() > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, receiver.Shutdown(context.Background()), "failed shutting down metrics receiver")

	actualMetrics := consumer.AllMetrics()[0]
	expectedFile := filepath.Join("testdata", "integration", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err, "failed reading expected metrics")

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues(), scrapertest.IgnoreResourceAttributeValue("aerospike.node.name")))

	// now do a run in cluster mode
	cfg.CollectClusterMetrics = true

	consumer = new(consumertest.MetricsSink)
	settings = componenttest.NewNopReceiverCreateSettings()
	receiver, err = f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	time.Sleep(time.Second / 2)
	require.NoError(t, receiver.Start(context.Background(), componenttest.NewNopHost()), "failed starting metrics receiver")

	require.Eventually(t, func() bool {
		return consumer.DataPointCount() > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, receiver.Shutdown(context.Background()), "failed shutting down metrics receiver")

	actualMetrics = consumer.AllMetrics()[0]
	expectedFile = filepath.Join("testdata", "integration", "expected.json")
	expectedMetrics, err = golden.ReadMetrics(expectedFile)
	require.NoError(t, err, "failed reading expected metrics")

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues(), scrapertest.IgnoreResourceAttributeValue("aerospike.node.name")))

}
