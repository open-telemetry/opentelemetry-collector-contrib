// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package zookeeperreceiver

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

const zookeeperPort = "2181"

func TestIntegration(t *testing.T) {
	t.Run("3.4.13", integrationTest("3.4.13", "zookeeper:3.4.13", false))
	t.Run("3.5.10", integrationTest("3.5.10", "zookeeper:3.5.10", false))
	t.Run("3.5.10-standalone", integrationTest("3.5.10-standalone", "docker.io/library/zookeeper:3.5.10", true))
}

func integrationTest(name string, image string, standalone bool) func(*testing.T) {
	return scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image: image,
				Env: map[string]string{
					"ZOO_4LW_COMMANDS_WHITELIST": "srvr,mntr,ruok",
					"ZOO_STANDALONE_ENABLED":     fmt.Sprintf("%t", standalone),
				},
				ExposedPorts: []string{zookeeperPort},
				WaitingFor:   wait.ForListeningPort(zookeeperPort),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, zookeeperPort))
			}),
		scraperinttest.WithExpectedFile(
			filepath.Join("testdata", "integration", fmt.Sprintf("expected-%s.yaml", name))),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run
}
