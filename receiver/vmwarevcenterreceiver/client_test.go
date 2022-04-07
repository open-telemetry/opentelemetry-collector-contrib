package vmwarevcenterreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestSimulatorCluster(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		finder := find.NewFinder(c)
		pm := performance.NewManager(c)
		// vsManager, err := vsan.NewClient(ctx, c)
		// require.NoError(t, err)

		clusters, err := finder.ClusterComputeResourceList(ctx, "*")
		require.NoError(t, err)
		require.NotEmpty(t, clusters, 0)

		for _, c := range clusters {
			// cluster properties
			var objC mo.ClusterComputeResource
			c.Properties(ctx, c.Reference(), []string{"summary"}, &objC)
			totalMem := objC.Summary.GetComputeResourceSummary().TotalMemory
			require.Greater(t, totalMem, int64(0))

			// host collection
			hosts, err := c.Hosts(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, hosts)

			// vsan Collection appeared to not work out of the box
			// startTime := time.Now().Add(-10 * time.Minute)
			// endTime := time.Now().Add(-1 * time.Minute)
			// querySpec := []vsanTypes.VsanPerfQuerySpec{
			// 	{
			// 		EntityRefId: "host-domclient:*",
			// 		StartTime:   &startTime,
			// 		EndTime:     &endTime,
			// 	},
			// }
			// cRef := c.Reference()
			// results, err := vsManager.VsanPerfQueryPerf(ctx, &cRef, querySpec)
			// require.NoError(t, err)
			// require.NotEmpty(t, results)

			// datastores
			dss, err := c.Datastores(ctx)
			require.NoError(t, err)
			for _, ds := range dss {
				var moDS mo.Datastore
				ds.Properties(ctx, ds.Reference(), []string{"summary"}, &moDS)
				capacity := moDS.Summary.Capacity
				require.Greater(t, capacity, int64(0))

				qs := []types.PerfQuerySpec{}
				startTime := time.Now().Add(-10 * time.Minute)
				endTime := time.Now().Add(-1 * time.Minute)
				qs = append(qs, types.PerfQuerySpec{
					Entity:    ds.Reference(),
					StartTime: &startTime,
					EndTime:   &endTime,
					MetricId:  []types.PerfMetricId{},
				})
				results, err := pm.Query(ctx, qs)
				require.NoError(t, err)
				require.NotEmpty(t, results)
			}
		}
	})
}
