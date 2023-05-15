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
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	mongoDBPort = "27017/tcp"
)

var (
	LPUSetupScript = []string{"/lpu.sh"}
	setupScript    = []string{"/setup.sh"}

	test4_0 = testCase{
		name:   "4_0",
		script: setupScript,
		container: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.mongodb.4_0",
			},
			ExposedPorts: []string{mongoDBPort},
			WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
		},
		cfgMod: func(cfg *Config, endpoint string) {
			cfg.MetricsBuilderConfig.Metrics.MongodbLockAcquireTime.Enabled = false
			cfg.Hosts = []confignet.NetAddr{
				{
					Endpoint: endpoint,
				},
			}
			cfg.Insecure = true
		},
	}

	test4_0LPU = testCase{
		name:   "4_4.lpu",
		script: LPUSetupScript,
		container: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.mongodb.4_4.lpu",
			},
			ExposedPorts: []string{mongoDBPort},
			WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
		},
		cfgMod: func(cfg *Config, endpoint string) {
			cfg.Username = "otelu"
			cfg.Password = "otelp"
			cfg.Hosts = []confignet.NetAddr{
				{
					Endpoint: endpoint,
				},
			}
			cfg.Insecure = true
		},
	}

	test5_0 = testCase{
		name:   "5_0",
		script: setupScript,
		container: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.mongodb.5_0",
			},
			ExposedPorts: []string{mongoDBPort},
			WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
		},
		cfgMod: func(cfg *Config, endpoint string) {
			cfg.Hosts = []confignet.NetAddr{
				{
					Endpoint: endpoint,
				},
			}
			cfg.Insecure = true
		},
	}

	compareOpts = []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
	}
)

type testCase struct {
	name      string
	container testcontainers.ContainerRequest
	script    []string
	cfgMod    func(defaultCfg *Config, endpoint string)
}

func TestMongodbIntegration(t *testing.T) {
	t.Run("4.0", test4_0.run)
	t.Run("4.0LPU", test4_0LPU.run)
	t.Run("5.0", test5_0.run)
}

func (tt testCase) run(t *testing.T) {
	t.Parallel()
	container, endpoint := getContainer(t, tt.container, tt.script)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.CollectionInterval = 10 * time.Second
	tt.cfgMod(cfg, endpoint)

	consumer := new(consumertest.MetricsSink)
	settings := receivertest.NewNopCreateSettings()
	rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	expectedFile := filepath.Join("testdata", "integration", fmt.Sprintf("expected.%s.yaml", tt.name))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.Eventually(t, scraperinttest.EqualsLatestMetrics(expectedMetrics, consumer, compareOpts), 2*time.Minute, 1*time.Second)
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest, script []string) (testcontainers.Container, string) {
	require.NoError(t, req.Validate())

	ctx := context.Background()

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)

	code, _, err := container.Exec(context.Background(), script)
	require.NoError(t, err)
	require.Equal(t, 0, code)

	err = container.Start(context.Background())
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, mongoDBPort)
	require.Nil(t, err)

	hostIP, err := container.Host(ctx)
	require.Nil(t, err)

	mongoDBEndpoint := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())

	return container, mongoDBEndpoint
}
