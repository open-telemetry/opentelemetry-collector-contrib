// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
		name: "4_0",
		container: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.mongodb.4_0",
			},
			ExposedPorts: []string{mongoDBPort},
			WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
			LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
				PostStarts: []testcontainers.ContainerHook{
					scraperinttest.RunScript(setupScript),
				},
			}},
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
		name: "4_4.lpu",
		container: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.mongodb.4_4.lpu",
			},
			ExposedPorts: []string{mongoDBPort},
			WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
			LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
				PostStarts: []testcontainers.ContainerHook{
					scraperinttest.RunScript(LPUSetupScript),
				},
			}},
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
		name: "5_0",
		container: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.mongodb.5_0",
			},
			ExposedPorts: []string{mongoDBPort},
			WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
			LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
				PostStarts: []testcontainers.ContainerHook{
					scraperinttest.RunScript(setupScript),
				},
			}},
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
	cfgMod    func(defaultCfg *Config, endpoint string)
}

func TestMongodbIntegration(t *testing.T) {
	t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16273")
	t.Run("4.0", test4_0.run)
	t.Run("4.0LPU", test4_0LPU.run)
	t.Run("5.0", test5_0.run)
}

func (tt testCase) run(t *testing.T) {
	t.Parallel()
	container, endpoint := getContainer(t, tt.container)
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

	mappedPort, err := container.MappedPort(ctx, mongoDBPort)
	require.Nil(t, err)

	hostIP, err := container.Host(ctx)
	require.Nil(t, err)

	mongoDBEndpoint := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())

	return container, mongoDBEndpoint
}
