// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func TestIntegrationESX(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		pw, set := simulator.DefaultLogin.Password()
		require.True(t, set)
		cfg := &Config{
			Endpoint: fmt.Sprintf("%s://%s", c.URL().Scheme, c.URL().Host),
			Username: simulator.DefaultLogin.Username(),
			Password: pw,
			TLSClientSetting: configtls.TLSClientSetting{
				Insecure: true,
			},
			MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		}
		s := session.NewManager(c)

		scraper := newVmwareVcenterScraper(zap.NewNop(), cfg, receivertest.NewNopCreateSettings())
		scraper.client.moClient = &govmomi.Client{
			Client:         c,
			SessionManager: s,
		}
		require.NoError(t, scraper.client.EnsureConnection(context.Background()))
		scraper.client.vimDriver = c
		scraper.client.finder = find.NewFinder(c)
		// Performance metrics rely on time based publishing so this is inherently flaky for an
		// integration test, so setting the performance manager to nil to not attempt to compare
		// performance metrcs. Coverage for this is encompassed in ./scraper_test.go
		scraper.client.pm = nil
		err := scraper.Start(ctx, componenttest.NewNopHost())
		require.NoError(t, err)

		metrics, err := scraper.scrape(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, metrics)

		goldenPath := filepath.Join("testdata", "metrics", "integration-metrics.yaml")
		expectedMetrics, err := golden.ReadMetrics(goldenPath)
		require.NoError(t, err)
		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, metrics,
			// the simulator will auto assign which host a VM is on, so it will be inconsistent which vm is on which host
			pmetrictest.IgnoreResourceAttributeValue("vcenter.host.name"),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues()))

		err = scraper.Shutdown(ctx)
		require.NoError(t, err)
	})
}
