// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package haproxyreceiver

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	cfgPath, err := filepath.Abs(filepath.Join("testdata", "haproxy.cfg"))
	require.NoError(t, err)
	haproxyPort := "8404"
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "docker.io/library/haproxy:2.8.1",
				ExposedPorts: []string{haproxyPort},
				Files: []testcontainers.ContainerFile{{
					HostFilePath:      cfgPath,
					ContainerFilePath: "/usr/local/etc/haproxy/haproxy.cfg",
					FileMode:          700,
				}},
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond
				rCfg.Endpoint = fmt.Sprintf("http://%s:%s/stats", ci.Host(t), ci.MappedPort(t, haproxyPort))
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreResourceAttributeValue("haproxy.addr"),
		),
	).Run(t)
}
