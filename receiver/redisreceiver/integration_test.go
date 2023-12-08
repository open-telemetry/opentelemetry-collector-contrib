// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package redisreceiver

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const redisPort = "6379"

func TestOlderIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "redis:6.0.3",
				ExposedPorts: []string{redisPort},
				WaitingFor:   wait.ForListeningPort(redisPort),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, redisPort))
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.ChangeResourceAttributeValue("server.address", func(_ string) string {
				return "localhost"
			}),
			pmetrictest.ChangeResourceAttributeValue("server.port", func(_ string) string {
				return redisPort
			}),
		),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", "expected-old.yaml")),
	).Run(t)
}

func TestLatestIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "redis:7.2.3",
				ExposedPorts: []string{redisPort},
				WaitingFor:   wait.ForListeningPort(redisPort),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, redisPort))
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.ChangeResourceAttributeValue("server.address", func(_ string) string {
				return "localhost"
			}),
			pmetrictest.ChangeResourceAttributeValue("server.port", func(_ string) string {
				return redisPort
			}),
		),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", "expected-latest.yaml")),
	).Run(t)
}
func TestClusterIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(testcontainers.ContainerRequest{
			ExposedPorts: []string{redisPort},
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.cluster",
			},
		}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, redisPort))
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.ChangeResourceAttributeValue("server.address", func(_ string) string {
				return "localhost"
			}),
			pmetrictest.ChangeResourceAttributeValue("server.port", func(_ string) string {
				return redisPort
			}),
		),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", "expected-cluster.yaml")),
	).Run(t)
}
