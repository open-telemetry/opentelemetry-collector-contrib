package mysqlreceiver

import (
	"context"
	"net"
	"path"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestMySqlIntegration(t *testing.T) {
	t.Run("Running mysql version 5.7", func(t *testing.T) {
		container := getContainer(t, containerRequest5_7)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Endpoint = net.JoinHostPort(hostname, "3307")
		cfg.Username = "otel"
		cfg.Password = "otel"

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
		require.NoError(t, rcvr.Shutdown(context.Background()))

		require.Equal(t, len(metadata.M.Names()), metrics.Len())
	})

	t.Run("Running mysql version 8.0", func(t *testing.T) {
		container := getContainer(t, containerRequest8_0)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Endpoint = net.JoinHostPort(hostname, "3306")
		cfg.Username = "otel"
		cfg.Password = "otel"

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
		require.NoError(t, rcvr.Shutdown(context.Background()))

		require.Equal(t, len(metadata.M.Names()), metrics.Len())

	})
}

var (
	containerRequest5_7 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join(".", "testdata", "dockerfiles"),
			Dockerfile: "Dockerfile.mysql.5_7",
		},
		ExposedPorts: []string{"3307:3306"},
		WaitingFor: wait.ForListeningPort("3306").
			WithStartupTimeout(2 * time.Minute),
	}
	containerRequest8_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join(".", "testdata", "dockerfiles"),
			Dockerfile: "Dockerfile.mysql.8_0",
		},
		ExposedPorts: []string{"3306:3306"},
		WaitingFor: wait.ForListeningPort("3306").
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

	code, err := container.Exec(context.Background(), []string{"/setup.sh"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	return container
}
