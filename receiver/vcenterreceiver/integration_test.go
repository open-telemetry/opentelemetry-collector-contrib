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
	_ "github.com/vmware/govmomi/vsan/simulator"
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
			MetricsConfig: &MetricsConfig{
				TLSClientSetting: configtls.TLSClientSetting{
					Insecure: true,
				},
				Settings: metadata.DefaultMetricsSettings(),
			},
		}
		s := session.NewManager(c)

		scraper := newVmwareVcenterScraper(zap.NewNop(), cfg)
		scraper.client.moClient = &govmomi.Client{
			Client:         c,
			SessionManager: s,
		}
		scraper.client.vimDriver = c
		scraper.client.finder = find.NewFinder(c)

		rcvr := &vcenterReceiver{
			config:  cfg,
			scraper: scraper,
		}

		err := rcvr.Start(ctx, componenttest.NewNopHost())
		require.NoError(t, err)

		sc, ok := rcvr.scraper.(*vcenterMetricScraper)
		require.True(t, ok)
		metrics, err := sc.scrape(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, metrics)

		goldenPath := filepath.Join("testdata", "metrics", "expected.json")
		expectedMetrics, err := golden.ReadMetrics(goldenPath)
		err = scrapertest.CompareMetrics(expectedMetrics, metrics)
		require.NoError(t, err)

		err = rcvr.Shutdown(ctx)
		require.NoError(t, err)
	})
}
