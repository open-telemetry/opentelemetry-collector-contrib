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

package vmwarevcenterreceiver

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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vmwarevcenterreceiver/internal/metadata"
)

func TestScrape_NoVsan(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := &vmwareVcenterClient{
			cfg: NewFactory().CreateDefaultConfig().(*Config),
			moClient: &govmomi.Client{
				SessionManager: session.NewManager(c),
				Client:         c,
			},
			vimDriver: c,
			finder:    finder,
		}
		scraper := &vcenterMetricScraper{
			client:      client,
			mb:          metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
			logger:      zap.NewNop(),
			vsanEnabled: false,
		}
		metrics, err := scraper.scrape(ctx)
		require.NoError(t, err)
		require.NotEqual(t, metrics.MetricCount(), 0)

		goldenPath := filepath.Join("testdata", "metrics", "expected.json")
		expectedMetrics, err := golden.ReadMetrics(goldenPath)
		require.NoError(t, err)

		scrapertest.CompareMetrics(expectedMetrics, metrics)
	})
}
