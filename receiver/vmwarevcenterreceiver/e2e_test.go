package vmwarevcenterreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
)

func TestEndtoEnd(t *testing.T) {
	sim := simulator.ESX()
	defer sim.Remove()

	sim.Run(func(ctx context.Context, c *vim25.Client) error {
		cfg := &Config{
			MetricsConfig: &MetricsConfig{
				TLSClientSetting: configtls.TLSClientSetting{
					Insecure: true,
				},
			},
		}
		s := session.NewManager(c)

		scraper := newVmwareVcenterScraper(zap.NewNop(), cfg)
		scraper.client.moClient = &govmomi.Client{
			Client:         c,
			SessionManager: s,
		}
		rcvr := &vcenterReceiver{
			config:  cfg,
			scraper: scraper,
		}

		err := rcvr.Start(ctx, componenttest.NewNopHost())
		require.NoError(t, err)

		err = rcvr.Shutdown(ctx)
		require.NoError(t, err)
		return nil
	})
}
