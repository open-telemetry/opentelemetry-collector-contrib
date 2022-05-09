package aerospikereceiver_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/containertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestAerospikeIntegration(t *testing.T) {
	t.Parallel()

	ct := containertest.New(t)
	container := ct.StartImage("docker.io/amd64/aerospike:ce-6.0.0.1", containertest.WithPortReady(3000))

	f := aerospikereceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*aerospikereceiver.Config)
	cfg.Endpoint = container.AddrForPort(3000)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond

	consumer := new(consumertest.MetricsSink)
	settings := componenttest.NewNopReceiverCreateSettings()
	receiver, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, receiver.Start(context.Background(), componenttest.NewNopHost()), "failed starting metrics receiver")

	require.Eventually(t, func() bool {
		return consumer.DataPointCount() > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, receiver.Shutdown(context.Background()), "failed shutting down metrics receiver")

	actualMetrics := consumer.AllMetrics()[0]
	expectedFile := filepath.Join("testdata", "integration", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err, "failed reading expected metrics")

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues()))
}
