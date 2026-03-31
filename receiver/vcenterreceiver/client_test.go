// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestDatacenters(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			vm:        vm,
		}
		dcs, err := client.Datacenters(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, dcs)
	})
}

func TestDatastores(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		dss, err := client.Datastores(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, dss)
	})
}

func TestEmptyDatastores(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Datastore = 0
	vpx.Machine = 0
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		dss, err := client.Datastores(ctx, dc.Reference())
		require.NoError(t, err)
		require.Empty(t, dss)
	}, vpx)
}

func TestComputeResources(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		crs, err := client.ComputeResources(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, crs)
	})
}

func TestComputeResourcesWithStandalone(t *testing.T) {
	esx := simulator.ESX()
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		crs, err := client.ComputeResources(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, crs)
	}, esx)
}

func TestHostSystems(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		hss, err := client.HostSystems(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, hss)
	})
}

func TestEmptyHostSystems(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Host = 0
	vpx.ClusterHost = 0
	vpx.Machine = 0
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		hss, err := client.HostSystems(ctx, dc.Reference())
		require.NoError(t, err)
		require.Empty(t, hss)
	}, vpx)
}

func TestResourcePools(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		rps, err := client.ResourcePools(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, rps)
	})
}

func TestVMs(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		vms, err := client.VMs(ctx, dc.Reference())
		require.NoError(t, err)
		require.NotEmpty(t, vms)
	})
}

func TestEmptyVMs(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Machine = 0
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		vm := view.NewManager(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
			vm:        vm,
		}
		dc, err := finder.DefaultDatacenter(ctx)
		require.NoError(t, err)
		vms, err := client.VMs(ctx, dc.Reference())
		require.NoError(t, err)
		require.Empty(t, vms)
	}, vpx)
}

func TestPerfMetricsQuery(t *testing.T) {
	esx := simulator.ESX()
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		pm := performance.NewManager(c)
		m := view.NewManager(c)
		finder := find.NewFinder(c)
		client := vcenterClient{
			vimDriver: c,
			vm:        m,
			pm:        pm,
			finder:    finder,
		}
		hs, err := finder.DefaultHostSystem(ctx)
		require.NoError(t, err)

		spec := types.PerfQuerySpec{Format: string(types.PerfFormatNormal), IntervalId: int32(20)}
		metrics, err := client.PerfMetricsQuery(ctx, spec, hostPerfMetricList, []types.ManagedObjectReference{hs.Reference()})
		require.NoError(t, err)
		require.NotEmpty(t, metrics.resultsByRef)
	}, esx)
}

func TestDatacenterInventoryListObjects(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Datacenter = 2
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
		}
		dcs, err := client.DatacenterInventoryListObjects(ctx)
		require.NoError(t, err)
		require.Len(t, dcs, 2)
	}, vpx)
}

func TestResourcePoolInventoryListObjects(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Datacenter = 2
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
		}
		dcs, err := finder.DatacenterList(ctx, "*")
		require.NoError(t, err)
		rps, err := client.ResourcePoolInventoryListObjects(ctx, dcs)
		require.NoError(t, err)
		require.NotEmpty(t, rps)
	}, vpx)
}

func TestVAppInventoryListObjects(t *testing.T) {
	// Currently skipping as the Simulator has no vApps by default and setting
	// vApps appears to be broken
	t.Skip()
	vpx := simulator.VPX()
	vpx.Datacenter = 2
	vpx.App = 2
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
		}
		dcs, err := finder.DatacenterList(ctx, "*")
		require.NoError(t, err)
		vApps, err := client.VAppInventoryListObjects(ctx, dcs)
		require.NoError(t, err)
		require.NotEmpty(t, vApps)
	}, vpx)
}

func TestEmptyVAppInventoryListObjects(t *testing.T) {
	vpx := simulator.VPX()
	vpx.Datacenter = 2
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		client := vcenterClient{
			vimDriver: c,
			finder:    finder,
		}
		dcs, err := finder.DatacenterList(ctx, "*")
		require.NoError(t, err)
		vApps, err := client.VAppInventoryListObjects(ctx, dcs)
		require.NoError(t, err)
		require.Empty(t, vApps)
	}, vpx)
}

func TestSessionReestablish(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		sm := session.NewManager(c)
		pw, _ := simulator.DefaultLogin.Password()
		client := vcenterClient{
			vimDriver: c,
			cfg: &Config{
				Username: simulator.DefaultLogin.Username(),
				Password: configopaque.String(pw),
				Endpoint: fmt.Sprintf("%s://%s", c.URL().Scheme, c.URL().Host),
				ClientConfig: configtls.ClientConfig{
					Insecure: true,
				},
			},
			sessionManager: sm,
		}
		err := sm.Logout(ctx)
		require.NoError(t, err)

		connected, err := client.sessionManager.SessionIsActive(ctx)
		require.NoError(t, err)
		require.False(t, connected)

		err = client.EnsureConnection(ctx)
		require.NoError(t, err)

		connected, err = client.sessionManager.SessionIsActive(ctx)
		require.NoError(t, err)
		require.True(t, connected)
	})
}
