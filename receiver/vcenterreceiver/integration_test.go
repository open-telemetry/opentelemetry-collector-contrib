// Copyright  The OpenTelemetry Authors
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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func TestEndtoEnd_ESX(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		cfg := &Config{
			TLSClientSetting: configtls.TLSClientSetting{
				Insecure: true,
			},
			Metrics: metadata.DefaultMetricsSettings(),
		}
		s := session.NewManager(c)

		scraper := newVmwareVcenterScraper(zap.NewNop(), cfg, componenttest.NewNopReceiverCreateSettings())
		scraper.client.moClient = &govmomi.Client{
			Client:         c,
			SessionManager: s,
		}
		scraper.client.vimDriver = c
		scraper.client.finder = find.NewFinder(c)
		// Performance metrics rely on time based publishing so this is inherently flaky for a
		// integration test, so setting the performance manager to nil to not attempt to compare
		// performance metrcs. Coverage for this is encompassed in ./scraper_test.go
		scraper.client.pm = nil
		err := scraper.Start(ctx, componenttest.NewNopHost())
		require.NoError(t, err)

		metrics, err := scraper.scrape(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, metrics)

		// the vcsim will auto assign the VM to one of the listed hosts, so this is a way to ignore the host.name for those vm metrics
		// please see #10129
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			if _, ok := metrics.ResourceMetrics().At(i).Resource().Attributes().Get("vcenter.host.name"); ok {
				metrics.ResourceMetrics().At(i).Resource().Attributes().Remove("vcenter.host.name")
				metrics.ResourceMetrics().At(i).Resource().Attributes().InsertString("vcenter.host.name", "DC0_C0_H0")
			}
		}

		goldenPath := filepath.Join("testdata", "metrics", "integration-metrics.json")
		expectedMetrics, err := golden.ReadMetrics(goldenPath)
		require.NoError(t, err)
		require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, metrics, scrapertest.IgnoreMetricValues()))

		err = scraper.Shutdown(ctx)
		require.NoError(t, err)
	})
}
