// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	t.Parallel()
	networkName := "new-network"
	ctx := context.Background()
	newNetwork, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:           networkName,
			CheckDuplicate: true,
		},
	})
	if err != nil {
		require.NoError(t, err)
	}
	defer func() {
		require.NoError(t, newNetwork.Remove(ctx))
	}()

	masterContainer := getContainer(t, testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.flink-master",
		},
		Hostname:     "flink-master",
		Networks:     []string{networkName},
		ExposedPorts: []string{"8080:8080", "8081:8081"},
		WaitingFor:   waitStrategy{endpoint: "http://localhost:8081/jobmanager/metrics"},
		LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
			PostStarts: []testcontainers.ContainerHook{
				scraperinttest.RunScript([]string{"/setup.sh"}),
			},
		}},
	})

	workerContainer := getContainer(t, testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.flink-worker",
		},
		Hostname: "worker",
		Networks: []string{networkName},
	})
	defer func() {
		require.NoError(t, masterContainer.Terminate(context.Background()))
	}()
	defer func() {
		require.NoError(t, workerContainer.Terminate(context.Background()))
	}()

	hostname, err := masterContainer.Host(context.Background())
	require.NoError(t, err)

	// Required to start the taskmanager
	ws := waitStrategy{"http://localhost:8081/taskmanagers/metrics"}
	err = ws.waitFor(context.Background(), "")
	require.NoError(t, err)

	// Required to prevent empty value jobs call
	ws = waitStrategy{endpoint: "http://localhost:8081/jobs"}
	err = ws.waitFor(context.Background(), "")
	require.NoError(t, err)

	// Required to prevent empty value for job, operator and task metrics call
	ws = waitStrategy{endpoint: "http://localhost:8081/jobs/metrics"}
	err = ws.waitFor(context.Background(), "")
	require.NoError(t, err)

	// Override function to return deterministic field
	defer func() { osHostname = os.Hostname }()
	osHostname = func() (string, error) { return "job-localhost", nil }

	// Override function to return deterministic field
	defer func() { taskmanagerHost = strings.Split }()
	taskmanagerHost = func(id string, sep string) []string { return []string{"taskmanager-localhost"} }

	// Override function to return deterministic field
	defer func() { taskmanagerID = reflect }()
	taskmanagerID = func(id string) string { return "taskmanagerID" }

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond
	cfg.Endpoint = fmt.Sprintf("http://%s", net.JoinHostPort(hostname, "8081"))

	consumer := new(consumertest.MetricsSink)
	settings := receivertest.NewNopCreateSettings()
	rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventuallyf(t, func() bool {
		return consumer.DataPointCount() > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, rcvr.Shutdown(context.Background()))

	actualMetrics := consumer.AllMetrics()[0]
	expectedFile := filepath.Join("testdata", "integration", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

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

type waitStrategy struct {
	endpoint string
}

func (ws waitStrategy) WaitUntilReady(ctx context.Context, st wait.StrategyTarget) error {
	if err := wait.ForListeningPort("8081").
		WithStartupTimeout(2*time.Minute).
		WaitUntilReady(ctx, st); err != nil {
		return err
	}

	hostname, err := st.Host(ctx)
	if err != nil {
		return err
	}

	return ws.waitFor(ctx, hostname)
}

// waitFor waits until an endpoint is ready with an id response.
func (ws waitStrategy) waitFor(ctx context.Context, _ string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return fmt.Errorf("server startup problem")
		case <-ticker.C:
			resp, err := http.Get(ws.endpoint)
			if err != nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}

			if resp.Body.Close() != nil {
				continue
			}

			// The server needs a moment to generate some stats
			if strings.Contains(string(body), "id") {
				return nil
			}
		}
	}
}
