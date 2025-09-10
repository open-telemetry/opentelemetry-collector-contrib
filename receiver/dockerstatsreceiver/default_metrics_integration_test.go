// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package dockerstatsreceiver

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/filter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	if runtime.GOOS == "darwin" && os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on Darwin GH runners: test requires Docker service")
	}

	// Start a docker container to ensure container metrics are available
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image: "docker.io/library/nginx:1.17",
				Name:  "dockerstatsreceiver-test",
			},
		),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = 100 * time.Millisecond
				rCfg.ResourceAttributes.ContainerName.MetricsInclude = []filter.Config{{Strict: "dockerstatsreceiver-test"}}
			},
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceAttributeValue("container.hostname"),
			pmetrictest.IgnoreResourceAttributeValue("container.id"),
			pmetrictest.IgnoreResourceAttributeValue("container.image.name"),
			pmetrictest.IgnoreResourceAttributeValue("container.runtime"),
			pmetrictest.IgnoreMetricAttributeValue("device_major", "container.blockio.io_service_bytes_recursive"),
			pmetrictest.IgnoreMetricAttributeValue("device_minor", "container.blockio.io_service_bytes_recursive"),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()),
	).Run(t)
}
