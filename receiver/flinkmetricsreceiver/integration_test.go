// Copyright  The OpenTelemetry Authors
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

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"context"
	"fmt"
	"io/ioutil"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestFlinkIntegration(t *testing.T) {
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

	// required to start the StateMachineExample job
	code, err := masterContainer.Exec(context.Background(), []string{"/setup.sh"})
	require.NoError(t, err)
	require.Equal(t, 0, code)

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
	settings := componenttest.NewNopReceiverCreateSettings()
	rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventuallyf(t, func() bool {
		return consumer.DataPointCount() > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, rcvr.Shutdown(context.Background()))

	actualMetrics := consumer.AllMetrics()[0]
	expectedFile := filepath.Join("testdata", "integration", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues()))
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

			body, err := ioutil.ReadAll(resp.Body)
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
