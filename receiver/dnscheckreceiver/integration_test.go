// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package dnscheckreceiver

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	corefilePath, err := filepath.Abs(filepath.Join("testdata", "integration", "Corefile"))
	require.NoError(t, err)
	zonefilePath, err := filepath.Abs(filepath.Join("testdata", "integration", "example.com.zone"))
	require.NoError(t, err)
	dnsPort := "53/udp"

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "coredns/coredns:1.11.3",
				Cmd:          []string{"-conf", "/Corefile"},
				ExposedPorts: []string{dnsPort},
				Files: []testcontainers.ContainerFile{
					{
						HostFilePath:      corefilePath,
						ContainerFilePath: "/Corefile",
						FileMode:          0o644,
					},
					{
						HostFilePath:      zonefilePath,
						ContainerFilePath: "/etc/coredns/example.com.zone",
						FileMode:          0o644,
					},
				},
				WaitingFor: wait.ForListeningPort(dnsPort).WithStartupTimeout(30 * time.Second),
			},
		),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.ControllerConfig.CollectionInterval = 100 * time.Millisecond
				rCfg.DNSServers = []DNSServerConfig{
					{
						Endpoint: net.JoinHostPort(ci.Host(t), ci.MappedPort(t, dnsPort)),
						Network:  "udp",
						Timeout:  5 * time.Second,
					},
				}
				rCfg.Hostnames = []HostnameConfig{
					{Name: "example.com", RecordType: "A"},
				}
			},
		),
		scraperinttest.WithCompareTimeout(30*time.Second),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues("dnscheck.duration"),
			pmetrictest.IgnoreResourceAttributeValue("dns.server"),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
