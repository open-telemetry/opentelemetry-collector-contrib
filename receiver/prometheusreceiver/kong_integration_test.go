// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package prometheusreceiver // Copyright The OpenTelemetry Authors

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const kongPort = "18001"

func TestKongIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "kong"),
					Dockerfile: "Dockerfile",
				},
				ExposedPorts: []string{kongPort + ":" + "8001"},
				WaitingFor:   wait.ForLog(".*connected to events broker.*").AsRegexp(),
			}),
		scraperinttest.AllowHardcodedHostPort(),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				cm, err := confmaptest.LoadConf(filepath.Join("testdata", "kong", "config.yaml"))
				require.NoError(t, err)
				require.NoError(t, cm.Unmarshal(cfg))
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "kong", "expected.yaml")),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricAttributeValue("node_id"),
			pmetrictest.IgnoreMetricAttributeValue("pid"),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreSubsequentDataPoints(),
			pmetrictest.IgnoreSummaryDataPointValueAtQuantileSliceOrder(),
			pmetrictest.IgnoreMetricAttributeValue("version", "kong_node_info"),
		),
	).Run(t)
}
