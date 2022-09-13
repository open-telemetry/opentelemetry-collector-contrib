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
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v6"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/containertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"
)

func populateMetrics(t *testing.T, host *as.Host) {
	clientPolicy := as.NewClientPolicy()
	clientPolicy.Timeout = 60 * time.Second
	clientPolicy.MinConnectionsPerNode = 50
	c, err := as.NewClientWithPolicyAndHost(clientPolicy, host)
	require.NoError(t, err)

	ns := "test"
	set := "integration"

	// write 100 records to get some memory usage
	for i := 0; i < 100; i++ {
		key, err := as.NewKey(ns, "integrations", i)
		require.NoError(t, err, "failed to create key")

		bins := as.BinMap{
			"bin1": i,
			"bin2": i,
		}

		err = c.Put(nil, key, bins)
		require.NoError(t, err, "failed to write record")
	}

	queryPolicy := as.NewQueryPolicy()
	queryPolicyShort := as.NewQueryPolicy()
	queryPolicyShort.ShortQuery = true

	var writePolicy *as.WritePolicy = nil

	execStatement := as.NewStatement(ns, "integrations")
	piStatement := as.NewStatement(ns, "integrations", "bin1")

	// perform a basic primary index query
	_, err = c.Query(queryPolicy, piStatement)
	require.NoError(t, err, "failed to execute basic PI query")

	// aggregation query on primary index
	_, err = c.QueryAggregate(queryPolicy, piStatement, "test_funcs", "func1") // TODO make the test UDFs
	require.NoError(t, err, "failed to execute aggregation PI query")

	// ops query on primary index
	ops := as.GetBinOp("bin1") // TODO add more op types
	_, err = c.QueryExecute(queryPolicy, writePolicy, execStatement, ops)
	require.NoError(t, err, "failed to execute ops PI query")

	// perform a basic short primary index query
	_, err = c.Query(queryPolicyShort, piStatement)
	require.NoError(t, err, "failed to execute basic short PI query")

	// create secondary index for SI queries
	c.CreateIndex(writePolicy, ns, set, "sitest", "bin2", as.NUMERIC)
	siStatement := as.NewStatement(ns, "integrations", "bin2")

	// perform a basic secondary index query
	_, err = c.Query(queryPolicy, siStatement)
	require.NoError(t, err, "failed to execute basic SI query")

	// aggregation query on secondary index
	_, err = c.QueryAggregate(queryPolicy, siStatement, "test_funcs", "func1") // TODO make the test UDFs
	require.NoError(t, err, "failed to execute aggregation SI query")

	// ops query on secondary index
	ops = as.GetBinOp("bin2") // TODO add more op types
	_, err = c.QueryExecute(queryPolicy, writePolicy, execStatement, ops)
	require.NoError(t, err, "failed to execute ops SI query")

	// perform a basic short secondary index query
	_, err = c.Query(queryPolicyShort, siStatement)
	require.NoError(t, err, "failed to execute basic short SI query")
}

func TestAerospikeIntegration(t *testing.T) {
	t.Parallel()

	ct := containertest.New(t)
	container := ct.StartImage("aerospike:ce-6.0.0.1", containertest.WithPortReady(3000))

	// time.Sleep(time.Second * 50)
	host := container.AddrForPort(3000)
	time.Sleep(time.Second * 2)

	ip, portStr, err := net.SplitHostPort(host)
	require.NoError(t, err)

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	asHost := as.NewHost(ip, port)
	populateMetrics(t, asHost)
	time.Sleep(time.Second / 2)

	f := aerospikereceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*aerospikereceiver.Config)
	cfg.Endpoint = host
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
