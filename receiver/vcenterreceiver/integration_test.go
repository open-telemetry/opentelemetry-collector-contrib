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

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegrationESX(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		pw, set := simulator.DefaultLogin.Password()
		require.True(t, set)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.CollectionInterval = 2 * time.Second
		cfg.Endpoint = fmt.Sprintf("%s://%s", c.URL().Scheme, c.URL().Host)
		cfg.Username = simulator.DefaultLogin.Username()
		cfg.Password = pw
		cfg.TLSClientSetting = configtls.TLSClientSetting{
			Insecure: true,
		}

		s := session.NewManager(c)
		newVcenterClient = func(cfg *Config) *vcenterClient {
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
			// performance metrcs. Coverage for this is encompassed in ./scraper_test.go
			client.pm = nil
			return client
		}
		defer func() {
			newVcenterClient = defaultNewVcenterClient
		}()

		consumer := new(consumertest.MetricsSink)
		settings := receivertest.NewNopCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")

		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		defer func() {
			require.NoError(t, rcvr.Shutdown(context.Background()))
		}()

		goldenPath := filepath.Join("testdata", "metrics", "integration-metrics.yaml")
		expectedMetrics, err := golden.ReadMetrics(goldenPath)
		require.NoError(t, err)

		compareOpts := []pmetrictest.CompareMetricsOption{
			// the simulator will auto assign which host a VM is on, so it will be inconsistent which vm is on which host
			pmetrictest.IgnoreResourceAttributeValue("vcenter.host.name"),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues()}

		require.Eventually(t, scraperinttest.EqualsLatestMetrics(expectedMetrics, consumer, compareOpts), 30*time.Second, time.Second)
	})
}
