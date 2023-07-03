// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package elasticsearchreceiver

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

const elasticPort = "9200"

func TestIntegration(t *testing.T) {
	t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/19755")
	t.Run("7.9.3", integrationTest("7_9_3"))
	t.Run("7.16.3", integrationTest("7_16_3"))
}

func integrationTest(name string) func(*testing.T) {
	dockerFile := fmt.Sprintf("Dockerfile.elasticsearch.%s", name)
	expectedFile := fmt.Sprintf("expected.%s.yaml", name)
	return scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
					Dockerfile: dockerFile,
				},
				ExposedPorts: []string{elasticPort},
				WaitingFor:   wait.ForListeningPort(elasticPort).WithStartupTimeout(2 * time.Minute),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = 2 * time.Second
				rCfg.Endpoint = fmt.Sprintf("http://%s:%s", ci.Host(t), ci.MappedPort(t, elasticPort))
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", expectedFile)),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceAttributeValue("elasticsearch.node.name"),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreResourceMetricsOrder(),
		),
	).Run
}
