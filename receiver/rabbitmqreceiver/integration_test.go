// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package rabbitmqreceiver

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

const rabbitmqPort = "15672"

func TestRabbitmqIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
					Dockerfile: "Dockerfile.rabbitmq",
				},
				ExposedPorts: []string{rabbitmqPort},
				WaitingFor:   wait.ForListeningPort(rabbitmqPort).WithStartupTimeout(time.Minute),
				LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
					PostStarts: []testcontainers.ContainerHook{
						scraperinttest.RunScript([]string{"./setup.sh"}),
					},
				}},
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("http://%s:%s", ci.Host(t), ci.MappedPort(t, rabbitmqPort))
				rCfg.Username = "otelu"
				rCfg.Password = "otelp"
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceAttributeValue("rabbitmq.node.name"),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues(),
		),
	).Run(t)
}
