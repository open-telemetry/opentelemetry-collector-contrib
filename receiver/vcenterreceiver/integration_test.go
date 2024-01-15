// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		pw, set := simulator.DefaultLogin.Password()
		require.True(t, set)

		s := session.NewManager(c)
		oldNewVcenterClientFunc := newVcenterClient
		newVcenterClient = func(cfg *Config) client {
			client := &vcenterClient{
				cfg: cfg,
				moClient: &govmomi.Client{
					Client:         c,
					SessionManager: s,
				},
			}
			require.NoError(t, client.EnsureConnection(context.Background()))
			client.vimDriver = c
			client.finder = find.NewFinder(c)
			// Performance metrics rely on time based publishing so this is inherently flaky for an
			// integration test, so setting the performance manager to nil to not attempt to compare
			// performance metrics. Coverage for this is encompassed in ./scraper_test.go
			client.pm = nil
			return client
		}
		defer func() {
			newVcenterClient = oldNewVcenterClientFunc
		}()

		scraperinttest.NewIntegrationTest(
			NewFactory(),
			scraperinttest.WithCustomConfig(
				func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
					rCfg := cfg.(*Config)
					rCfg.CollectionInterval = 2 * time.Second
					rCfg.Endpoint = fmt.Sprintf("%s://%s", c.URL().Scheme, c.URL().Host)
					rCfg.Username = simulator.DefaultLogin.Username()
					rCfg.Password = configopaque.String(pw)
					rCfg.TLSClientSetting = configtls.TLSClientSetting{
						Insecure: true,
					}
				}),
			scraperinttest.WithCompareOptions(
				pmetrictest.IgnoreResourceAttributeValue("vcenter.host.name"),
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreMetricValues(),
			),
		).Run(t)
	})
}

func TestIntegrationWithCaching(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		pw, set := simulator.DefaultLogin.Password()
		require.True(t, set)

		s := session.NewManager(c)
		oldNewVcenterClientFunc := newVcenterClient
		newVcenterClient = func(cfg *Config) client {
			client := &vcenterClient{
				cfg: cfg,
				moClient: &govmomi.Client{
					Client:         c,
					SessionManager: s,
				},
			}
			require.NoError(t, client.EnsureConnection(context.Background()))
			client.vimDriver = c
			client.finder = find.NewFinder(c)
			// Performance metrics rely on time based publishing so this is inherently flaky for an
			// integration test, so setting the performance manager to nil to not attempt to compare
			// performance metrics. Coverage for this is encompassed in ./scraper_test.go
			client.pm = nil
			cache := ttlcache.NewCache()
			_ = cache.SetTTL(5 * time.Minute)
			cache.SkipTTLExtensionOnHit(true)
			return &cachingClient{
				delegate: client,
				cache:    cache,
			}
		}
		defer func() {
			newVcenterClient = oldNewVcenterClientFunc
		}()

		scraperinttest.NewIntegrationTest(
			NewFactory(),
			scraperinttest.WithCustomConfig(
				func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
					rCfg := cfg.(*Config)
					rCfg.CollectionInterval = 2 * time.Second
					rCfg.Endpoint = fmt.Sprintf("%s://%s", c.URL().Scheme, c.URL().Host)
					rCfg.Username = simulator.DefaultLogin.Username()
					rCfg.Password = configopaque.String(pw)
					rCfg.TLSClientSetting = configtls.TLSClientSetting{
						Insecure: true,
					}
				}),
			scraperinttest.WithCompareOptions(
				pmetrictest.IgnoreResourceAttributeValue("vcenter.host.name"),
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreMetricValues(),
			),
		).Run(t)
	})
}

func TestVcsim(t *testing.T) {
	vcsaPort := "8989"
	metricNames := []string{
		"vcenter.host.disk.latency.max",
		"vcenter.host.disk.throughput",
		"vcenter.host.network.packet.count",
		"vcenter.host.network.throughput",
		"vcenter.host.network.usage",
		"vcenter.vm.network.packet.count",
		"vcenter.vm.network.throughput",
		"vcenter.vm.network.usage",
		"vcenter.vm.network.packet.count",
	}
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "vmware/vcsim:v0.34.2",
				Cmd:          []string{"-vm", "1", "-dc", "5", "-cluster", "2", "-l", ":8989"},
				ExposedPorts: []string{vcsaPort},
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.ScraperControllerSettings.CollectionInterval = 5 * time.Second
				rCfg.ScraperControllerSettings.InitialDelay = 20 * time.Second
				rCfg.Endpoint = fmt.Sprintf("https://%s:%s", ci.Host(t), ci.MappedPort(t, vcsaPort))
				rCfg.Username = "username"
				rCfg.Password = "password"
				rCfg.TLSClientSetting.InsecureSkipVerify = true
				rCfg.TLSClientSetting.Insecure = true
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", "expected_vcsim.yaml")),
		scraperinttest.WriteExpected(),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues(metricNames...),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreResourceAttributeValue("vcenter.vm.id"),
			pmetrictest.IgnoreResourceAttributeValue("vcenter.vm.name"),
			pmetrictest.IgnoreResourceAttributeValue("vcenter.host.name"),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
		),
	).Run(t)
}
