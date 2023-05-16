// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package mysqlreceiver

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const mysqlPort = "3306"

func TestMySQLIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("testdata", "integration"),
				Dockerfile: "Dockerfile.mysql",
			},
			ExposedPorts: []string{mysqlPort},
			WaitingFor: wait.ForListeningPort(mysqlPort).
				WithStartupTimeout(2 * time.Minute),
			LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
				PostStarts: []testcontainers.ContainerHook{
					scraperinttest.RunScript([]string{"/setup.sh"}),
				},
			}},
		},
		scraperinttest.WithCustomConfig(
			func(cfg component.Config, host string, mappedPort scraperinttest.MappedPortFunc) {
				port := mappedPort(mysqlPort)
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				rCfg.Endpoint = net.JoinHostPort(host, port)
				rCfg.Username = "otel"
				rCfg.Password = "otel"
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceAttributeValue("mysql.instance.endpoint"),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
