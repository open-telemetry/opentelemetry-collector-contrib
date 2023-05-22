// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	sparkPort = "4040/tcp"
)

var (
	containerRequest = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.apache-spark",
		},
		ExposedPorts: []string{sparkPort},
		WaitingFor:   wait.ForListeningPort(sparkPort).WithStartupTimeout(2 * time.Minute),
	}
)

type testCase struct {
	name      string
	container testcontainers.ContainerRequest
}

func TestApacheSparkIntegration(t *testing.T) {
	testCases := []testCase{
		{
			name:      "apache-spark",
			container: containerRequest,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, tt.run)
	}
}

func (tt testCase) run(t *testing.T) {
	t.Parallel()
	container, endpoint := getContainer(t, tt.container)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.CollectionInterval = 3 * time.Second
	cfg.Endpoint = endpoint

	consumer := new(consumertest.MetricsSink)
	settings := receivertest.NewNopCreateSettings()
	rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	// Wait for multiple collections, in case the first represents partially started system
	require.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 1
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, rcvr.Shutdown(context.Background()))
	actualMetrics := consumer.AllMetrics()[1]

	expectedFile := filepath.Join("testdata", "integration", fmt.Sprintf("expected.%s.yaml", tt.name))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricValues(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceAttributeValue("spark.application.id"), pmetrictest.IgnoreResourceAttributeValue("spark.application.name"), pmetrictest.IgnoreMetricAttributeValue("active"), pmetrictest.IgnoreMetricAttributeValue("complete"), pmetrictest.IgnoreMetricAttributeValue("failed"), pmetrictest.IgnoreMetricAttributeValue("pending"), pmetrictest.IgnoreMetricDataPointsOrder()))
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest) (testcontainers.Container, string) {
	require.NoError(t, req.Validate())

	ctx := context.Background()

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)

	err = container.Start(context.Background())
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, sparkPort)
	require.Nil(t, err)

	hostIP, err := container.Host(ctx)
	require.Nil(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", hostIP, mappedPort.Port())

	return container, endpoint
}
