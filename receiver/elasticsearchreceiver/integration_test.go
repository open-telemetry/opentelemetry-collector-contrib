// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package elasticsearchreceiver

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const elasticPort = "9200"

func TestIntegration(t *testing.T) {
	t.Run("7.9.3", integrationTest("7_9_3"))
	t.Run("7.16.3", integrationTest("7_16_3"))
}

func integrationTest(name string) func(*testing.T) {
	dockerFile := fmt.Sprintf("Dockerfile.elasticsearch.%s", name)
	expectedFile := fmt.Sprintf("expected.%s.yaml", name)
	return func(t *testing.T) {
		scraperinttest.NewIntegrationTest(
			NewFactory(),
			scraperinttest.WithContainerRequest(
				testcontainers.ContainerRequest{
					FromDockerfile: testcontainers.FromDockerfile{
						Context:    filepath.Join("testdata", "integration"),
						Dockerfile: dockerFile,
					},
					ExposedPorts: []string{elasticPort + "/tcp"},
					WaitingFor:   wait.ForHTTP("/").WithPort(nat.Port(elasticPort)).WithStartupTimeout(2 * time.Minute),
				}),
			scraperinttest.WithCustomConfig(
				func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
					// Log container readiness as suggested
					t.Logf("Checking container readiness...")

					rCfg := cfg.(*Config)
					rCfg.CollectionInterval = 2 * time.Second
					// Use net.JoinHostPort for proper IPv6 handling
					// Use elasticPort (just "9200") for MappedPort, NOT "9200/tcp"
					mappedPort := ci.MappedPort(t, elasticPort)
					endpoint := fmt.Sprintf("http://%s", net.JoinHostPort(ci.Host(t), mappedPort))
					t.Logf("Elasticsearch endpoint: %s", endpoint)
					rCfg.Endpoint = endpoint
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
			scraperinttest.FailOnErrorLogs(),
			scraperinttest.WithCompareTimeout(2*time.Minute),
		).Run(t)
	}
}
