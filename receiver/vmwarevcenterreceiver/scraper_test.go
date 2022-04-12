package vmwarevcenterreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"go.uber.org/zap"

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
	})
}
