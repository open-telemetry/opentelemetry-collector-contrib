// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

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

const sparkPort = "4040"

func TestApacheSparkIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
					Dockerfile: "Dockerfile.apache-spark",
				},
				ExposedPorts: []string{sparkPort},
				WaitingFor:   wait.ForListeningPort(sparkPort).WithStartupTimeout(2 * time.Minute),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.ScraperControllerSettings.CollectionInterval = 3 * time.Second
				rCfg.Endpoint = fmt.Sprintf("http://%s:%s", ci.Host(t), ci.MappedPort(t, sparkPort))
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreResourceAttributeValue("spark.application.id"),
			pmetrictest.IgnoreResourceAttributeValue("spark.application.name"),
			pmetrictest.IgnoreMetricAttributeValue("active", "spark.stage.status"),
			pmetrictest.IgnoreMetricAttributeValue("complete", "spark.stage.status"),
			pmetrictest.IgnoreMetricAttributeValue("failed", "spark.stage.status"),
			pmetrictest.IgnoreMetricAttributeValue("pending", "spark.stage.status"),
			pmetrictest.IgnoreMetricDataPointsOrder(),
		),
	).Run(t)
}
