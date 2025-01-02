// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package valkeyreceiver

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const valkeyPort = "6379"

func TestIntegration(t *testing.T) {
	t.Run("8", integrationTest("8.0"))
}

func integrationTest(version string) func(*testing.T) {
	return scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        fmt.Sprintf("valkey/valkey:%s", version),
				ExposedPorts: []string{valkeyPort},
				WaitingFor:   wait.ForListeningPort(valkeyPort).WithStartupTimeout(2 * time.Minute),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.MetricsBuilderConfig.ResourceAttributes.ServerPort.Enabled = true
				rCfg.MetricsBuilderConfig.ResourceAttributes.ServerAddress.Enabled = true
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, valkeyPort))
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "all-metrics", "output-metrics.yaml")),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.ChangeResourceAttributeValue("server.address", func(_ string) string {
				return "localhost"
			}),
			pmetrictest.ChangeResourceAttributeValue("server.port", func(_ string) string {
				return valkeyPort
			}),
		),
	).Run
}
