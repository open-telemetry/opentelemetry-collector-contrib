// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package mongodbreceiver

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const mongoPort = "27017"

func TestIntegration(t *testing.T) {
	t.Run("4.0", integrationTest("4_0", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("5.0", integrationTest("5_0", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("4.4lpu", integrationTest("4_4lpu", []string{"/lpu.sh"}, func(cfg *Config) {
		cfg.Username = "otelu"
		cfg.Password = "otelp"
	}))
}

func integrationTest(name string, script []string, cfgMod func(*Config)) func(*testing.T) {
	dockerFile := fmt.Sprintf("Dockerfile.mongodb.%s", name)
	expectedFile := fmt.Sprintf("expected.%s.yaml", name)
	return scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
					Dockerfile: dockerFile,
				},
				ExposedPorts: []string{mongoPort},
				WaitingFor:   wait.ForListeningPort(mongoPort).WithStartupTimeout(time.Minute),
				LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
					PostStarts: []testcontainers.ContainerHook{
						scraperinttest.RunScript(script),
					},
				}},
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				cfgMod(rCfg)
				rCfg.CollectionInterval = 2 * time.Second
				rCfg.MetricsBuilderConfig.Metrics.MongodbLockAcquireTime.Enabled = false
				rCfg.Hosts = []confignet.NetAddr{
					{
						Endpoint: fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, mongoPort)),
					},
				}
				rCfg.Insecure = true
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", expectedFile)),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run
}
