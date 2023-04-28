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
) 

const (
	sparkPort = "4040/tcp"
)

var (
	setupScript      = []string{"/setup.sh"}
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
	script    []string
	cfgMod    func(defaultCfg *Config, endpoint string)
}

func TestApacheSparkIntegration(t *testing.T) {
	testCases := []testCase{
		{
			name:      "apache-spark",
			script:    setupScript,
			container: containerRequest,
			cfgMod: func(cfg *Config, endpoint string) {
				cfg.Endpoint = endpoint
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, tt.run)
	}
}

func (tt testCase) run(t *testing.T) {
	t.Parallel()
	container, _ := getContainer(t, tt.container, tt.script)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()

	// f := NewFactory()
	// cfg := f.CreateDefaultConfig().(*Config)
	// cfg.CollectionInterval = 10 * time.Second
	// tt.cfgMod(cfg, endpoint)

	// consumer := new(consumertest.MetricsSink)
	// settings := receivertest.NewNopCreateSettings()
	// rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	// require.NoError(t, err, "failed creating metrics receiver")

	// require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	// // Wait for multiple collections, in case the first represents partially started system
	// require.Eventuallyf(t, func() bool {
	// 	return len(consumer.AllMetrics()) > 1
	// }, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	// require.NoError(t, rcvr.Shutdown(context.Background()))
	// actualMetrics := consumer.AllMetrics()[1]

	// expectedFile := filepath.Join("testdata", "integration", fmt.Sprintf("expected.%s.yaml", tt.name))
	// expectedMetrics, err := golden.ReadMetrics(expectedFile)
	// require.NoError(t, err)

	// require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricValues(),
	// 	pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest, _ []string) (testcontainers.Container, string) {
	require.NoError(t, req.Validate())

	ctx := context.Background()

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)

	// code, _, err := container.Exec(ctx, script)
	// require.NoError(t, err)
	// require.Equal(t, 0, code)

	err = container.Start(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, sparkPort)
	require.Nil(t, err)

	hostIP, err := container.Host(ctx)
	require.Nil(t, err)

	endpoint := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())

	return container, endpoint
}
